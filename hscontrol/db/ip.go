package db

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"net/netip"
	"sync"

	"github.com/juanfont/headscale/hscontrol/types"
	"github.com/juanfont/headscale/hscontrol/util"
	"github.com/rs/zerolog/log"
	"go4.org/netipx"
	"gorm.io/gorm"
)

// IPAllocator is a singleton responsible for allocating
// IP addresses for nodes and making sure the same
// address is not handed out twice. There can only be one
// and it needs to be created before any other database
// writes occur.
type IPAllocator struct {
	mu sync.Mutex

	prefix4 *netip.Prefix
	prefix6 *netip.Prefix

	// strategy used for handing out IP addresses.
	strategy types.IPAllocationStrategy

	// Set of all IPs handed out.
	// This might not be in sync with the database,
	// but it is more conservative. If saves to the
	// database fails, the IP will be allocated here
	// until the next restart of Headscale.
	usedIPs netipx.IPSetBuilder
}

// NewIPAllocator returns a new IPAllocator singleton which
// can be used to hand out unique IP addresses within the
// provided IPv4 and IPv6 prefix. It needs to be created
// when headscale starts and needs to finish its read
// transaction before any writes to the database occur.
func NewIPAllocator(
	db *HSDatabase,
	prefix4, prefix6 *netip.Prefix,
	strategy types.IPAllocationStrategy,
) (*IPAllocator, error) {
	ret := IPAllocator{
		prefix4: prefix4,
		prefix6: prefix6,

		strategy: strategy,
	}

	var v4s []sql.NullString
	var v6s []sql.NullString

	if db != nil {
		err := db.Read(func(rx *gorm.DB) error {
			return rx.Model(&types.Node{}).Pluck("ipv4", &v4s).Error
		})
		if err != nil {
			return nil, fmt.Errorf("reading IPv4 addresses from database: %w", err)
		}

		err = db.Read(func(rx *gorm.DB) error {
			return rx.Model(&types.Node{}).Pluck("ipv6", &v6s).Error
		})
		if err != nil {
			return nil, fmt.Errorf("reading IPv6 addresses from database: %w", err)
		}
	}

	var ips netipx.IPSetBuilder

	// Add network and broadcast addrs to used pool so they
	// are not handed out to nodes.
	if prefix4 != nil {
		network4, broadcast4 := util.GetIPPrefixEndpoints(*prefix4)
		ips.Add(network4)
		ips.Add(broadcast4)
	}

	if prefix6 != nil {
		network6, broadcast6 := util.GetIPPrefixEndpoints(*prefix6)
		ips.Add(network6)
		ips.Add(broadcast6)
	}

	// Fetch all the IP Addresses currently handed out from the Database
	// and add them to the used IP set.
	for _, addrStr := range append(v4s, v6s...) {
		if addrStr.Valid {
			addr, err := netip.ParseAddr(addrStr.String)
			if err != nil {
				return nil, fmt.Errorf("parsing IP address from database: %w", err)
			}

			ips.Add(addr)
		}
	}

	// Build the initial IPSet to validate that we can use it.
	_, err := ips.IPSet()
	if err != nil {
		return nil, fmt.Errorf(
			"building initial IP Set: %w",
			err,
		)
	}

	ret.usedIPs = ips

	return &ret, nil
}

func (i *IPAllocator) Next(db *HSDatabase) (*netip.Addr, *netip.Addr, error) {
	if db == nil {
		i.mu.Lock()
		defer i.mu.Unlock()

		var err error
		var ret4 *netip.Addr
		var ret6 *netip.Addr

		if i.prefix4 != nil {
			ret4, err = i.next(i.prefix4, i.usedIPs)
			if err != nil {
				return nil, nil, fmt.Errorf("allocating IPv4 address: %w", err)
			}
		}

		if i.prefix6 != nil {
			ret6, err = i.next(i.prefix6, i.usedIPs)
			if err != nil {
				return nil, nil, fmt.Errorf("allocating IPv6 address: %w", err)
			}
		}

		return ret4, ret6, nil
	}

	var ret4, ret6 *netip.Addr
	err := db.Write(func(tx *gorm.DB) error {
		var v4s []sql.NullString
		var v6s []sql.NullString

		err := tx.Model(&types.Node{}).Pluck("ipv4", &v4s).Error
		if err != nil {
			return fmt.Errorf("reading IPv4 addresses from database: %w", err)
		}

		err = tx.Model(&types.Node{}).Pluck("ipv6", &v6s).Error
		if err != nil {
			return fmt.Errorf("reading IPv6 addresses from database: %w", err)
		}

		var ips netipx.IPSetBuilder

		if i.prefix4 != nil {
			network4, broadcast4 := util.GetIPPrefixEndpoints(*i.prefix4)
			ips.Add(network4)
			ips.Add(broadcast4)
		}

		if i.prefix6 != nil {
			network6, broadcast6 := util.GetIPPrefixEndpoints(*i.prefix6)
			ips.Add(network6)
			ips.Add(broadcast6)
		}

		for _, addrStr := range append(v4s, v6s...) {
			if addrStr.Valid {
				addr, err := netip.ParseAddr(addrStr.String)
				if err != nil {
					return fmt.Errorf("parsing IP address from database: %w", err)
				}

				ips.Add(addr)
			}
		}

		i.usedIPs = ips

		if i.prefix4 != nil {
			ret4, err = i.next(i.prefix4, ips)
			if err != nil {
				return fmt.Errorf("allocating IPv4 address: %w", err)
			}
		}

		if i.prefix6 != nil {
			ret6, err = i.next(i.prefix6, ips)
			if err != nil {
				return fmt.Errorf("allocating IPv6 address: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		return nil, nil, err
	}

	return ret4, ret6, nil
}

var ErrCouldNotAllocateIP = errors.New("failed to allocate IP")

func (i *IPAllocator) nextLocked(prefix *netip.Prefix) (*netip.Addr, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	return i.next(prefix, i.usedIPs)
}

func (i *IPAllocator) next(prefix *netip.Prefix, usedIPs netipx.IPSetBuilder) (*netip.Addr, error) {
	var err error
	var ip netip.Addr

	set, err := usedIPs.IPSet()
	if err != nil {
		return nil, err
	}

	switch i.strategy {
	case types.IPAllocationStrategySequential:
		// Find the highest allocated IP within the prefix from the current set
		var highestAllocatedIP netip.Addr
		for _, r := range set.Ranges() {
			for ipInSet := r.From(); prefix.Contains(ipInSet) && ipInSet.Compare(r.To()) <= 0; ipInSet = ipInSet.Next() {
				if highestAllocatedIP == (netip.Addr{}) || ipInSet.Compare(highestAllocatedIP) > 0 {
					highestAllocatedIP = ipInSet
				}
			}
		}

		currentIP := prefix.Addr().Next() // Start after network address
		if highestAllocatedIP != (netip.Addr{}) {
			currentIP = highestAllocatedIP.Next()
		}

		for {
			if !prefix.Contains(currentIP) {
				return nil, ErrCouldNotAllocateIP
			}
			if !set.Contains(currentIP) {
				ip = currentIP
				break
			}
			currentIP = currentIP.Next()
		}
	case types.IPAllocationStrategyRandom:
		for attempts := 0; attempts < 100; attempts++ {
			ip, err = randomNext(*prefix)
			if err != nil {
				return nil, fmt.Errorf("getting random IP: %w", err)
			}
			if !set.Contains(ip) {
				break
			}
		}
		if set.Contains(ip) {
			return nil, ErrCouldNotAllocateIP
		}
	}

	usedIPs.Add(ip)

	return &ip, nil
}

func randomNext(pfx netip.Prefix) (netip.Addr, error) {
	rang := netipx.RangeOfPrefix(pfx)
	fromIP, toIP := rang.From(), rang.To()

	var from, to big.Int

	from.SetBytes(fromIP.AsSlice())
	to.SetBytes(toIP.AsSlice())

	// Find the max, this is how we can do "random range",
	// get the "max" as 0 -> to - from and then add back from
	// after.
	tempMax := big.NewInt(0).Sub(&to, &from)

	out, err := rand.Int(rand.Reader, tempMax)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("generating random IP: %w", err)
	}

	valInRange := big.NewInt(0).Add(&from, out)

	ip, ok := netip.AddrFromSlice(valInRange.Bytes())
	if !ok {
		return netip.Addr{}, fmt.Errorf("generated ip bytes are invalid ip")
	}

	if !pfx.Contains(ip) {
		return netip.Addr{}, fmt.Errorf(
			"generated ip(%s) not in prefix(%s)",
			ip.String(),
			pfx.String(),
		)
	}

	return ip, nil
}

// BackfillNodeIPs will take a database transaction, and
// iterate through all of the current nodes in headscale
// and ensure it has IP addresses according to the current
// configuration.
// This means that if both IPv4 and IPv6 is set in the
// config, and some nodes are missing that type of IP,
// it will be added.
// If a prefix type has been removed (IPv4 or IPv6), it
// will remove the IPs in that family from the node.
func (db *HSDatabase) BackfillNodeIPs(i *IPAllocator) ([]string, error) {
	var err error
	var ret []string
	err = db.Write(func(tx *gorm.DB) error {
		if i == nil {
			return errors.New("backfilling IPs: ip allocator was nil")
		}

		log.Trace().Msgf("starting to backfill IPs")

		// Rebuild the IPSet from the database within the transaction
		var v4s []sql.NullString
		var v6s []sql.NullString

		err := tx.Model(&types.Node{}).Pluck("ipv4", &v4s).Error
		if err != nil {
			return fmt.Errorf("reading IPv4 addresses from database: %w", err)
		}

		err = tx.Model(&types.Node{}).Pluck("ipv6", &v6s).Error
		if err != nil {
			return fmt.Errorf("reading IPv6 addresses from database: %w", err)
		}

		var currentIPs netipx.IPSetBuilder

		if i.prefix4 != nil {
			network4, broadcast4 := util.GetIPPrefixEndpoints(*i.prefix4)
			currentIPs.Add(network4)
			currentIPs.Add(broadcast4)
		}

		if i.prefix6 != nil {
			network6, broadcast6 := util.GetIPPrefixEndpoints(*i.prefix6)
			currentIPs.Add(network6)
			currentIPs.Add(broadcast6)
		}

		for _, addrStr := range append(v4s, v6s...) {
			if addrStr.Valid {
				addr, err := netip.ParseAddr(addrStr.String)
				if err != nil {
					return fmt.Errorf("parsing IP address from database: %w", err)
				}

				currentIPs.Add(addr)
			}
		}

		nodes, err := ListNodes(tx)
		if err != nil {
			return fmt.Errorf("listing nodes to backfill IPs: %w", err)
		}

		for _, node := range nodes {
			log.Trace().Uint64("node.id", node.ID.Uint64()).Msg("checking if need backfill")

			changed := false
			// IPv4 prefix is set, but node ip is missing, alloc
			if i.prefix4 != nil && node.IPv4 == nil {
				ret4, err := i.next(i.prefix4, currentIPs)
				if err != nil {
					return fmt.Errorf("failed to allocate ipv4 for node(%d): %w", node.ID, err)
				}

				node.IPv4 = ret4
				changed = true
				ret = append(ret, fmt.Sprintf("assigned IPv4 %q to Node(%d) %q", ret4.String(), node.ID, node.Hostname))
			}

			// IPv6 prefix is set, but node ip is missing, alloc
			if i.prefix6 != nil && node.IPv6 == nil {
				ret6, err := i.next(i.prefix6, currentIPs)
				if err != nil {
					return fmt.Errorf("failed to allocate ipv6 for node(%d): %w", node.ID, err)
				}

				node.IPv6 = ret6
				changed = true
				ret = append(ret, fmt.Sprintf("assigned IPv6 %q to Node(%d) %q", ret6.String(), node.ID, node.Hostname))
			}

			// IPv4 prefix is not set, but node has IP, remove
			if i.prefix4 == nil && node.IPv4 != nil {
				ret = append(ret, fmt.Sprintf("removing IPv4 %q from Node(%d) %q", node.IPv4.String(), node.ID, node.Hostname))
				node.IPv4 = nil
				changed = true
			}

			// IPv6 prefix is not set, but node has IP, remove
			if i.prefix6 == nil && node.IPv6 != nil {
				ret = append(ret, fmt.Sprintf("removing IPv6 %q from Node(%d) %q", node.IPv6.String(), node.ID, node.Hostname))
				node.IPv6 = nil
				changed = true
			}

			if changed {
				err := tx.Save(node).Error
				if err != nil {
					return fmt.Errorf("saving node(%d) after adding IPs: %w", node.ID, err)
				}
			}
		}

		return nil
	})

	return ret, err
}
