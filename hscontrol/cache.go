package hscontrol

import (
	"net/netip"
	"sort"

	"github.com/juanfont/headscale/hscontrol/db"
	"github.com/juanfont/headscale/hscontrol/types"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
	"tailscale.com/types/key"
)

func (h *Headscale) updateRoutesCache() {
	var routes types.Routes
	var err error

	h.routesCacheMutex.Lock()
	defer h.routesCacheMutex.Unlock()

	routes, err = db.Read(h.db.DB, func(rx *gorm.DB) (types.Routes, error) {
		return db.GetRoutes(rx)
	})
	if err != nil {
		log.Error().Err(err).Msg("Error retrieving routes")
		return
	}
	h.routesCache = routes
}

func (h *Headscale) GetNodeRoutesFromCache(m *types.Node) []types.Route {
	h.routesCacheMutex.RLock()
	defer h.routesCacheMutex.RUnlock()

	var routes []types.Route
	for _, route := range h.routesCache {
		if route.NodeID == uint64(m.ID) {
			routes = append(routes, route)
		}
	}
	return routes
}
func (h *Headscale) getNodePrimaryRoutesFromCache(m *types.Node) []types.Route {
	h.routesCacheMutex.RLock()
	defer h.routesCacheMutex.RUnlock()

	var routes []types.Route
	for _, route := range h.routesCache {
		if route.NodeID == uint64(m.ID) && route.Advertised && route.Enabled && route.IsPrimary {
			routes = append(routes, route)
		}
	}
	return routes
}

// SaveNodeRoutesWithCache processes node routes with batch operations and updates cache
func (h *Headscale) SaveNodeRoutesWithCache(node *types.Node) error {
	_, err := h.db.SaveNodeRoutes(node)
	if err != nil {
		return err
	}

	// Update routes cache after successful route processing
	h.updateRoutesCache()
	return nil
}

func (h *Headscale) updatePeersCache() {
	var peers types.Nodes
	var err error

	h.peersCacheMutex.Lock()
	defer h.peersCacheMutex.Unlock()

	peers, err = h.db.ListNodes()
	if err != nil {
		log.Error().Err(err).Msg("Error retrieving list of machines")
		return
	}

	sort.Slice(peers, func(i, j int) bool { return peers[i].ID < peers[j].ID })
	h.peersCache = peers
}

func (h *Headscale) getPeersCache(pickoutNode *types.Node) types.Nodes {
	h.peersCacheMutex.RLock()
	defer h.peersCacheMutex.RUnlock()

	if pickoutNode != nil {
		found := make(types.Nodes, 0)

		for _, peer := range h.peersCache {
			if pickoutNode.ID != peer.ID {
				found = append(found, peer)
			}
		}
		return found
	}

	return h.peersCache
}

// GetNodesByAuthKeyIds finds a Node by AuthKeyIds and returns the Node struct.
func (h *Headscale) GetNodesByAuthKeyIds(authKeyIds []uint64) ([]*types.Node, error) {
	m := []*types.Node{}
	if result := h.db.DB.Where("auth_key_id IN (?)", authKeyIds).Find(&m); result.Error != nil {
		return nil, result.Error
	}

	return m, nil
}

// GetNodeByPreAuthKeyAndHostname finds a Node by its PreAuthKey And Hostname, and returns the Node struct.
func (h *Headscale) GetNodeByAuthKeyAndHostname(
	authKey string, hostname string,
) (*types.Node, error) {
	node := types.Node{}

	if result := h.db.DB.Preload("AuthKey").Preload("AuthKey.User").Preload("User").First(&node, "hostname = ? ", hostname); result.Error != nil {
		return nil, result.Error
	}

	if node.AuthKey.Key == authKey {

		return &node, nil
	} else {

		return nil, gorm.ErrRecordNotFound
	}
}

func (h *Headscale) GetNodeByAnyKeyAndAuthKeyAndHostname(
	machineKey key.MachinePublic, nodeKey key.NodePublic, oldNodeKey key.NodePublic, authKey string, hostname string,
) (*types.Node, bool, error) {
	node, err := h.db.GetNodeByAnyKey(machineKey, nodeKey, oldNodeKey)
	if err == nil {
		return node, false, nil
	}

	// Fallback to authKey + hostname
	node, err = h.GetNodeByAuthKeyAndHostname(authKey, hostname)
	if err == nil {
		return node, true, nil
	}

	return nil, false, err
}
func removePrefix(slice []netip.Prefix, tv netip.Prefix) []netip.Prefix {
	for i, v := range slice {
		if v == tv {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

func removePrefixes(slice []netip.Prefix, tvs []netip.Prefix) []netip.Prefix {
	for _, v := range tvs {
		slice = removePrefix(slice, v)
	}
	return slice
}
