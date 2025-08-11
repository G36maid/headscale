// nolint
package hscontrol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
	"tailscale.com/tailcfg"
	"tailscale.com/types/key"

	v1 "github.com/juanfont/headscale/gen/go/headscale/v1"
	"github.com/juanfont/headscale/hscontrol/db"
	"github.com/juanfont/headscale/hscontrol/policy"
	"github.com/juanfont/headscale/hscontrol/types"
	"github.com/juanfont/headscale/hscontrol/util"
)

type headscaleV1APIServer struct { // v1.HeadscaleServiceServer
	v1.UnimplementedHeadscaleServiceServer
	h *Headscale
}

func newHeadscaleV1APIServer(h *Headscale) v1.HeadscaleServiceServer {
	return headscaleV1APIServer{
		h: h,
	}
}

func (api headscaleV1APIServer) GetUser(
	ctx context.Context,
	request *v1.GetUserRequest,
) (*v1.GetUserResponse, error) {
	user, err := api.h.db.GetUser(request.GetName())
	if err != nil {
		return nil, err
	}

	return &v1.GetUserResponse{User: user.Proto()}, nil
}

func (api headscaleV1APIServer) CreateUser(
	ctx context.Context,
	request *v1.CreateUserRequest,
) (*v1.CreateUserResponse, error) {
	user, err := api.h.db.CreateUser(request.GetName())
	if err != nil {
		return nil, err
	}

	return &v1.CreateUserResponse{User: user.Proto()}, nil
}

func (api headscaleV1APIServer) RenameUser(
	ctx context.Context,
	request *v1.RenameUserRequest,
) (*v1.RenameUserResponse, error) {
	err := api.h.db.RenameUser(request.GetOldName(), request.GetNewName())
	if err != nil {
		return nil, err
	}

	user, err := api.h.db.GetUser(request.GetNewName())
	if err != nil {
		return nil, err
	}

	return &v1.RenameUserResponse{User: user.Proto()}, nil
}

func (api headscaleV1APIServer) DeleteUser(
	ctx context.Context,
	request *v1.DeleteUserRequest,
) (*v1.DeleteUserResponse, error) {
	err := api.h.db.DestroyUser(request.GetName())
	if err != nil {
		return nil, err
	}

	return &v1.DeleteUserResponse{}, nil
}

func (api headscaleV1APIServer) ListUsers(
	ctx context.Context,
	request *v1.ListUsersRequest,
) (*v1.ListUsersResponse, error) {
	users, err := api.h.db.ListUsers()
	if err != nil {
		return nil, err
	}

	response := make([]*v1.User, len(users))
	for index, user := range users {
		response[index] = user.Proto()
	}

	sort.Slice(response, func(i, j int) bool {
		return response[i].Id < response[j].Id
	})

	log.Trace().Caller().Interface("users", response).Msg("")

	return &v1.ListUsersResponse{Users: response}, nil
}

func (api headscaleV1APIServer) CreatePreAuthKey(
	ctx context.Context,
	request *v1.CreatePreAuthKeyRequest,
) (*v1.CreatePreAuthKeyResponse, error) {
	var expiration time.Time
	if request.GetExpiration() != nil {
		expiration = request.GetExpiration().AsTime()
	}

	for _, tag := range request.AclTags {
		err := validateTag(tag)
		if err != nil {
			return &v1.CreatePreAuthKeyResponse{
				PreAuthKey: nil,
			}, status.Error(codes.InvalidArgument, err.Error())
		}
	}

	preAuthKey, err := api.h.db.CreatePreAuthKey(
		request.GetUser(),
		request.GetReusable(),
		request.GetEphemeral(),
		&expiration,
		request.AclTags,
	)
	if err != nil {
		return nil, err
	}

	return &v1.CreatePreAuthKeyResponse{PreAuthKey: preAuthKey.Proto()}, nil
}

func (api headscaleV1APIServer) EnablePreAuthKeys(
	ctx context.Context,
	request *v1.PreAuthKeysRequest,
) (*v1.PreAuthKeysResponse, error) {
	log.Trace().
		Interface("request.SguUuids", request.SguUuids).
		Interface("request.ClientUuids", request.ClientUuids).
		Msg("Enable PreAuthKeys")

	if len(request.SguUuids) > 0 && len(request.ClientUuids) == 0 {
		return api.EnablePreAuthKeysBySguUuid(ctx, request)
	} else if len(request.SguUuids) == 0 && len(request.ClientUuids) > 0 {
		return api.EnablePreAuthKeysByClientUuids(ctx, request)
	} else {
		return &v1.PreAuthKeysResponse{}, nil
	}
}

func (api headscaleV1APIServer) EnablePreAuthKeysBySguUuid(
	ctx context.Context,
	request *v1.PreAuthKeysRequest,
) (*v1.PreAuthKeysResponse, error) {
	uuids := request.SguUuids

	preAuthKeys, _ := api.h.db.GetPreAuthKeysBySguUuids(uuids)

	api.h.db.EnablePreAuthKeys(preAuthKeys)

	return &v1.PreAuthKeysResponse{}, nil
}

func (api headscaleV1APIServer) EnablePreAuthKeysByClientUuids(
	ctx context.Context,
	request *v1.PreAuthKeysRequest,
) (*v1.PreAuthKeysResponse, error) {
	clientUuids := request.ClientUuids

	preAuthKeys, _ := api.h.db.GetPreAuthKeysByClientUuids(clientUuids)

	api.h.db.EnablePreAuthKeys(preAuthKeys)

	return &v1.PreAuthKeysResponse{}, nil
}

func (api headscaleV1APIServer) RemovePreAuthKeys(
	ctx context.Context,
	request *v1.PreAuthKeysRequest,
) (*v1.PreAuthKeysResponse, error) {
	log.Trace().
		Interface("request.SguUuids", request.SguUuids).
		Interface("request.ClientUuids", request.ClientUuids).
		Msg("Remove PreAuthKeys")

	if len(request.SguUuids) > 0 && len(request.ClientUuids) == 0 {
		return api.RemovePreAuthKeysBySguUuid(ctx, request)
	} else if len(request.SguUuids) == 0 && len(request.ClientUuids) > 0 {
		return api.RemovePreAuthKeysByClientUuids(ctx, request)
	} else {
		return &v1.PreAuthKeysResponse{}, nil
	}
}

func (api headscaleV1APIServer) RemovePreAuthKeysBySguUuid(
	ctx context.Context,
	request *v1.PreAuthKeysRequest,
) (*v1.PreAuthKeysResponse, error) {
	uuids := request.SguUuids

	preAuthKeys, _ := api.h.db.GetPreAuthKeysBySguUuids(uuids)

	api.h.db.DestroyPreAuthKeys(preAuthKeys)

	return &v1.PreAuthKeysResponse{}, nil
}

func (api headscaleV1APIServer) RemovePreAuthKeysByClientUuids(
	ctx context.Context,
	request *v1.PreAuthKeysRequest,
) (*v1.PreAuthKeysResponse, error) {
	clientUuids := request.ClientUuids

	preAuthKeys, _ := api.h.db.GetPreAuthKeysByClientUuids(clientUuids)

	if len(preAuthKeys) > 0 {
		api.h.db.DisablePreAuthKeys(preAuthKeys)

		var authKeyIdList []uint64
		for _, key := range preAuthKeys {
			authKeyIdList = append(authKeyIdList, key.ID)
		}
		nodes, _ := api.h.db.GetNodesByAuthKeyIds(authKeyIdList)

		// for _, node := range nodes {
		// 	expireRequest := &v1.ExpireNodeRequest{NodeId: uint64(node.ID)}
		// 	api.ExpireNode(ctx, expireRequest) //expire Nodes
		// }
		if len(nodes) > 0 {
			var nodeIds []uint64
			for _, node := range nodes {
				nodeIds = append(nodeIds, uint64(node.ID))
			}
			expireRequest := &v1.ExpireNodesRequest{
				ClientUuids: clientUuids,
			}
			api.ExpireNodes(ctx, expireRequest)
		}

		api.h.db.DestroyPreAuthKeys(preAuthKeys)

		go func() {
			time.Sleep(10 * time.Second)
			for _, node := range nodes {
				deleteRequest := &v1.DeleteNodeRequest{NodeId: uint64(node.ID)}
				api.DeleteNode(ctx, deleteRequest) //delete Nodes
			}
		}()

		log.Info().
			Strs("clientUuids", clientUuids).
			Interface("authKeyIdList", authKeyIdList).
			Interface("nodes", nodes).
			Msg("Remove PreAuthKeys by Client Uuids")
	}

	return &v1.PreAuthKeysResponse{}, nil
}

func (api headscaleV1APIServer) ExpirePreAuthKey(
	ctx context.Context,
	request *v1.ExpirePreAuthKeyRequest,
) (*v1.ExpirePreAuthKeyResponse, error) {
	err := api.h.db.Write(func(tx *gorm.DB) error {
		preAuthKey, err := db.GetPreAuthKey(tx, request.GetUser(), request.Key)
		if err != nil {
			return err
		}

		return db.ExpirePreAuthKey(tx, preAuthKey)
	})
	if err != nil {
		return nil, err
	}

	return &v1.ExpirePreAuthKeyResponse{}, nil
}

func (api headscaleV1APIServer) ListPreAuthKeys(
	ctx context.Context,
	request *v1.ListPreAuthKeysRequest,
) (*v1.ListPreAuthKeysResponse, error) {
	preAuthKeys, err := api.h.db.ListPreAuthKeys(request.GetUser())
	if err != nil {
		return nil, err
	}

	response := make([]*v1.PreAuthKey, len(preAuthKeys))
	for index, key := range preAuthKeys {
		response[index] = key.Proto()
	}

	sort.Slice(response, func(i, j int) bool {
		return response[i].Id < response[j].Id
	})

	return &v1.ListPreAuthKeysResponse{PreAuthKeys: response}, nil
}

func (api headscaleV1APIServer) LockPreAuthKeyTagLock(
	ctx context.Context,
	request *v1.PreAuthKeyTagLockRequest,
) (*v1.PreAuthKeyTagLockResponse, error) {
	log.Trace().
		Interface("request.SguUuid", request.SguUuid).
		Msg("Lock PreAuthKeyTagLock")
	err := api.h.db.SetPreAuthKeyTagLockBySguUuid(request.SguUuid, true)
	return &v1.PreAuthKeyTagLockResponse{}, err
}

func (api headscaleV1APIServer) UnlockPreAuthKeyTagLock(
	ctx context.Context,
	request *v1.PreAuthKeyTagLockRequest,
) (*v1.PreAuthKeyTagLockResponse, error) {
	log.Trace().
		Interface("request.SguUuid", request.SguUuid).
		Msg("Unlock PreAuthKeyTagLock")
	err := api.h.db.SetPreAuthKeyTagLockBySguUuid(request.SguUuid, false)
	return &v1.PreAuthKeyTagLockResponse{}, err
}

func (api headscaleV1APIServer) RegisterNode(
	ctx context.Context,
	request *v1.RegisterNodeRequest,
) (*v1.RegisterNodeResponse, error) {
	log.Trace().
		Str("user", request.GetUser()).
		Str("machine_key", request.GetKey()).
		Msg("Registering node")

	var mkey key.MachinePublic
	err := mkey.UnmarshalText([]byte(request.GetKey()))
	if err != nil {
		return nil, err
	}

	ipv4, ipv6, err := api.h.ipAlloc.Next(api.h.db)
	if err != nil {
		return nil, err
	}

	node, err := db.Write(api.h.db.DB, func(tx *gorm.DB) (*types.Node, error) {
		return db.RegisterNodeFromAuthCallback(
			tx,
			api.h.registrationCache,
			mkey,
			request.GetUser(),
			nil,
			util.RegisterMethodCLI,
			ipv4, ipv6,
		)
	})
	if err != nil {
		return nil, err
	}

	api.h.setLastStateChangeToNow()

	return &v1.RegisterNodeResponse{Node: node.Proto()}, nil
}

func (api headscaleV1APIServer) GetNode(
	ctx context.Context,
	request *v1.GetNodeRequest,
) (*v1.GetNodeResponse, error) {
	node, err := api.h.db.GetNodeByID(types.NodeID(request.GetNodeId()))
	if err != nil {
		return nil, err
	}

	resp := node.Proto()

	// Populate the online field based on
	// currently connected nodes.
	resp.Online = api.h.nodeNotifier.IsConnected(node.ID)

	return &v1.GetNodeResponse{Node: resp}, nil
}

func (api headscaleV1APIServer) SetTags(
	ctx context.Context,
	request *v1.SetTagsRequest,
) (*v1.SetTagsResponse, error) {
	for _, tag := range request.GetTags() {
		err := validateTag(tag)
		if err != nil {
			return nil, err
		}
	}

	node, err := db.Write(api.h.db.DB, func(tx *gorm.DB) (*types.Node, error) {
		err := db.SetTags(tx, types.NodeID(request.GetNodeId()), request.GetTags())
		if err != nil {
			return nil, err
		}

		return db.GetNodeByID(tx, types.NodeID(request.GetNodeId()))
	})
	if err != nil {
		return &v1.SetTagsResponse{
			Node: nil,
		}, status.Error(codes.InvalidArgument, err.Error())
	}

	api.h.setLastStateChangeToNow()

	ctx = types.NotifyCtx(ctx, "cli-settags", node.Hostname)
	api.h.nodeNotifier.NotifyWithIgnore(ctx, types.StateUpdate{
		Type:        types.StatePeerChanged,
		ChangeNodes: []types.NodeID{node.ID},
		Message:     "called from api.SetTags",
	}, node.ID)

	log.Trace().
		Str("node", node.Hostname).
		Strs("tags", request.GetTags()).
		Msg("Changing tags of node")

	return &v1.SetTagsResponse{Node: node.Proto()}, nil
}

func validateTag(tag string) error {
	if strings.Index(tag, "tag:") != 0 {
		return errors.New("tag must start with the string 'tag:'")
	}
	if strings.ToLower(tag) != tag {
		return errors.New("tag should be lowercase")
	}
	if len(strings.Fields(tag)) > 1 {
		return errors.New("tag should not contains space")
	}
	return nil
}

func (api headscaleV1APIServer) DeleteNode(
	ctx context.Context,
	request *v1.DeleteNodeRequest,
) (*v1.DeleteNodeResponse, error) {
	node, err := api.h.db.GetNodeByID(types.NodeID(request.GetNodeId()))
	if err != nil {
		return nil, err
	}

	changedNodes, err := api.h.db.DeleteNode(
		node,
		api.h.nodeNotifier.LikelyConnectedMap(),
	)
	if err != nil {
		return nil, err
	}

	api.h.setLastStateChangeToNow()

	ctx = types.NotifyCtx(ctx, "cli-deletenode", node.Hostname)
	api.h.nodeNotifier.NotifyAll(ctx, types.StateUpdate{
		Type:    types.StatePeerRemoved,
		Removed: []types.NodeID{node.ID},
	})

	if changedNodes != nil {
		api.h.nodeNotifier.NotifyAll(ctx, types.StateUpdate{
			Type:        types.StatePeerChanged,
			ChangeNodes: changedNodes,
		})
	}

	return &v1.DeleteNodeResponse{}, nil
}

func (api headscaleV1APIServer) ExpireNode(
	ctx context.Context,
	request *v1.ExpireNodeRequest,
) (*v1.ExpireNodeResponse, error) {
	now := time.Now()

	node, err := db.Write(api.h.db.DB, func(tx *gorm.DB) (*types.Node, error) {
		db.NodeSetExpiry(
			tx,
			types.NodeID(request.GetNodeId()),
			now,
		)

		return db.GetNodeByID(tx, types.NodeID(request.GetNodeId()))
	})
	if err != nil {
		return nil, err
	}

	api.h.setLastStateChangeToNow()

	ctx = types.NotifyCtx(ctx, "cli-expirenode-self", node.Hostname)
	api.h.nodeNotifier.NotifyByNodeID(
		ctx,
		types.StateUpdate{
			Type:        types.StateSelfUpdate,
			ChangeNodes: []types.NodeID{node.ID},
		},
		node.ID)

	ctx = types.NotifyCtx(ctx, "cli-expirenode-peers", node.Hostname)
	api.h.nodeNotifier.NotifyWithIgnore(
		ctx,
		types.StateUpdateExpire(node.ID, now),
		node.ID,
	)

	log.Trace().
		Str("node", node.Hostname).
		Time("expiry", *node.Expiry).
		Msg("node expired")

	return &v1.ExpireNodeResponse{Node: node.Proto()}, nil
}

func (api headscaleV1APIServer) ExpireNodes(
	ctx context.Context,
	request *v1.ExpireNodesRequest,
) (*v1.ExpireNodesResponse, error) {
	log.Trace().
		Interface("request.RecoveryInterval", request.RecoveryInterval).
		Interface("request.SguUuids", request.SguUuids).
		Interface("request.ClientUuids", request.ClientUuids).
		Msg("Expire Nodes expired")

	if len(request.SguUuids) > 0 && len(request.ClientUuids) == 0 {
		return api.ExpireNodesBySguUuid(ctx, request)
	} else if len(request.SguUuids) == 0 && len(request.ClientUuids) > 0 {
		return api.ExpireNodesByClientUuids(ctx, request)
	} else {
		return &v1.ExpireNodesResponse{}, nil
	}
}

func (api headscaleV1APIServer) ExpireNodesBySguUuid(
	ctx context.Context,
	request *v1.ExpireNodesRequest,
) (*v1.ExpireNodesResponse, error) {
	recoveryInterval := request.RecoveryInterval
	uuids := request.SguUuids

	preAuthKeys, _ := api.h.db.GetPreAuthKeysBySguUuids(uuids)

	if len(preAuthKeys) > 0 {
		api.h.db.DisablePreAuthKeys(preAuthKeys)

		var authKeyIdList []uint64
		for _, key := range preAuthKeys {
			authKeyIdList = append(authKeyIdList, key.ID)
		}
		nodes, _ := api.h.db.GetNodesByAuthKeyIds(authKeyIdList)

		//api.h.db.ExpireNode(nodes)
		for _, node := range nodes {
			expireRequest := &v1.ExpireNodeRequest{NodeId: uint64(node.ID)}
			api.ExpireNode(ctx, expireRequest) //expire Nodes
		}

		log.Info().
			Strs("Super Group User Uuid", uuids).
			Interface("authKeyIdList", authKeyIdList).
			Interface("nodes", nodes).
			Msg("Expire Node expired by SGU Uuid")

		if recoveryInterval > 0 {
			go func() {
				time.Sleep(time.Duration(recoveryInterval) * time.Second)
				api.h.db.EnablePreAuthKeys(preAuthKeys)
			}()
		}
	}
	return &v1.ExpireNodesResponse{SguUuids: uuids}, nil
}

func (api headscaleV1APIServer) ExpireNodesByClientUuids(
	ctx context.Context,
	request *v1.ExpireNodesRequest,
) (*v1.ExpireNodesResponse, error) {
	recoveryInterval := request.RecoveryInterval
	clientUuids := request.ClientUuids

	preAuthKeys, _ := api.h.db.GetPreAuthKeysByClientUuids(clientUuids)

	if len(preAuthKeys) > 0 {
		api.h.db.DisablePreAuthKeys(preAuthKeys)

		var authKeyIdList []uint64
		for _, key := range preAuthKeys {
			authKeyIdList = append(authKeyIdList, key.ID)
		}
		nodes, _ := api.h.db.GetNodesByAuthKeyIds(authKeyIdList)

		//api.h.db.ExpireNode(nodes)
		for _, node := range nodes {
			expireRequest := &v1.ExpireNodeRequest{NodeId: uint64(node.ID)}
			api.ExpireNode(ctx, expireRequest) //expire Nodes
		}

		log.Info().
			Strs("clientUuids", clientUuids).
			Interface("authKeyIdList", authKeyIdList).
			Interface("nodes", nodes).
			Msg("Expire Node expired by Client Uuids")
		if recoveryInterval > 0 {
			go func() {
				time.Sleep(time.Duration(recoveryInterval) * time.Second)
				api.h.db.EnablePreAuthKeys(preAuthKeys)
			}()
		}
	}
	return &v1.ExpireNodesResponse{ClientUuids: clientUuids}, nil
}

func (api headscaleV1APIServer) RenameNode(
	ctx context.Context,
	request *v1.RenameNodeRequest,
) (*v1.RenameNodeResponse, error) {
	node, err := db.Write(api.h.db.DB, func(tx *gorm.DB) (*types.Node, error) {
		err := db.RenameNode(
			tx,
			types.NodeID(request.GetNodeId()),
			request.GetNewName(),
		)
		if err != nil {
			return nil, err
		}

		return db.GetNodeByID(tx, types.NodeID(request.GetNodeId()))
	})
	if err != nil {
		return nil, err
	}

	api.h.setLastStateChangeToNow()

	ctx = types.NotifyCtx(ctx, "cli-renamenode", node.Hostname)
	api.h.nodeNotifier.NotifyWithIgnore(ctx, types.StateUpdate{
		Type:        types.StatePeerChanged,
		ChangeNodes: []types.NodeID{node.ID},
		Message:     "called from api.RenameNode",
	}, node.ID)

	log.Trace().
		Str("node", node.Hostname).
		Str("new_name", request.GetNewName()).
		Msg("node renamed")

	return &v1.RenameNodeResponse{Node: node.Proto()}, nil
}

func (api headscaleV1APIServer) ListNodes(
	ctx context.Context,
	request *v1.ListNodesRequest,
) (*v1.ListNodesResponse, error) {
	isLikelyConnected := api.h.nodeNotifier.LikelyConnectedMap()
	if request.GetUser() != "" {
		nodes, err := db.Read(api.h.db.DB, func(rx *gorm.DB) (types.Nodes, error) {
			return db.ListNodesByUser(rx, request.GetUser())
		})
		if err != nil {
			return nil, err
		}

		response := make([]*v1.Node, len(nodes))
		for index, node := range nodes {
			resp := node.Proto()

			// Populate the online field based on
			// currently connected nodes.
			if val, ok := isLikelyConnected.Load(node.ID); ok && val {
				resp.Online = true
			}

			response[index] = resp
		}

		return &v1.ListNodesResponse{Nodes: response}, nil
	}

	nodes, err := api.h.db.ListNodes()
	if err != nil {
		return nil, err
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})

	response := make([]*v1.Node, len(nodes))
	for index, node := range nodes {
		resp := node.Proto()

		// Populate the online field based on
		// currently connected nodes.
		if val, ok := isLikelyConnected.Load(node.ID); ok && val {
			resp.Online = true
		}

		validTags, invalidTags := api.h.ACLPolicy.TagsOfNode(
			node,
		)
		resp.InvalidTags = invalidTags
		resp.ValidTags = validTags
		response[index] = resp
	}

	return &v1.ListNodesResponse{Nodes: response}, nil
}

func (api headscaleV1APIServer) MoveNode(
	ctx context.Context,
	request *v1.MoveNodeRequest,
) (*v1.MoveNodeResponse, error) {
	node, err := api.h.db.GetNodeByID(types.NodeID(request.GetNodeId()))
	if err != nil {
		return nil, err
	}

	err = api.h.db.AssignNodeToUser(node, request.GetUser())
	if err != nil {
		return nil, err
	}

	api.h.setLastStateChangeToNow()

	return &v1.MoveNodeResponse{Node: node.Proto()}, nil
}

func (api headscaleV1APIServer) BackfillNodeIPs(
	ctx context.Context,
	request *v1.BackfillNodeIPsRequest,
) (*v1.BackfillNodeIPsResponse, error) {
	log.Trace().Msg("Backfill called")

	if !request.Confirmed {
		return nil, errors.New("not confirmed, aborting")
	}

	changes, err := api.h.db.BackfillNodeIPs(api.h.ipAlloc)
	if err != nil {
		return nil, err
	}

	api.h.setLastStateChangeToNow()

	return &v1.BackfillNodeIPsResponse{Changes: changes}, nil
}

func (api headscaleV1APIServer) GetRoutes(
	ctx context.Context,
	request *v1.GetRoutesRequest,
) (*v1.GetRoutesResponse, error) {
	routes, err := db.Read(api.h.db.DB, func(rx *gorm.DB) (types.Routes, error) {
		return db.GetRoutes(rx)
	})
	if err != nil {
		return nil, err
	}

	return &v1.GetRoutesResponse{
		Routes: types.Routes(routes).Proto(),
	}, nil
}

func (api headscaleV1APIServer) EnableRoute(
	ctx context.Context,
	request *v1.EnableRouteRequest,
) (*v1.EnableRouteResponse, error) {
	update, err := db.Write(api.h.db.DB, func(tx *gorm.DB) (*types.StateUpdate, error) {
		return db.EnableRoute(tx, request.GetRouteId())
	})
	if err != nil {
		return nil, err
	}

	if update != nil {
		ctx := types.NotifyCtx(ctx, "cli-enableroute", "unknown")
		api.h.nodeNotifier.NotifyAll(
			ctx, *update)
	}

	return &v1.EnableRouteResponse{}, nil
}

func (api headscaleV1APIServer) DisableRoute(
	ctx context.Context,
	request *v1.DisableRouteRequest,
) (*v1.DisableRouteResponse, error) {
	update, err := db.Write(api.h.db.DB, func(tx *gorm.DB) ([]types.NodeID, error) {
		return db.DisableRoute(
			tx,
			request.GetRouteId(),
			api.h.nodeNotifier.LikelyConnectedMap(),
		)
	})
	if err != nil {
		return nil, err
	}

	if update != nil {
		ctx := types.NotifyCtx(ctx, "cli-disableroute", "unknown")
		api.h.nodeNotifier.NotifyAll(ctx, types.StateUpdate{
			Type:        types.StatePeerChanged,
			ChangeNodes: update,
		})
	}

	return &v1.DisableRouteResponse{}, nil
}

func (api headscaleV1APIServer) GetNodeRoutes(
	ctx context.Context,
	request *v1.GetNodeRoutesRequest,
) (*v1.GetNodeRoutesResponse, error) {
	node, err := api.h.db.GetNodeByID(types.NodeID(request.GetNodeId()))
	if err != nil {
		return nil, err
	}

	routes, err := api.h.db.GetNodeRoutes(node)
	if err != nil {
		return nil, err
	}

	return &v1.GetNodeRoutesResponse{
		Routes: types.Routes(routes).Proto(),
	}, nil
}

func (api headscaleV1APIServer) DeleteRoute(
	ctx context.Context,
	request *v1.DeleteRouteRequest,
) (*v1.DeleteRouteResponse, error) {
	isConnected := api.h.nodeNotifier.LikelyConnectedMap()
	update, err := db.Write(api.h.db.DB, func(tx *gorm.DB) ([]types.NodeID, error) {
		return db.DeleteRoute(tx, request.GetRouteId(), isConnected)
	})
	if err != nil {
		return nil, err
	}

	if update != nil {
		ctx := types.NotifyCtx(ctx, "cli-deleteroute", "unknown")
		api.h.nodeNotifier.NotifyAll(ctx, types.StateUpdate{
			Type:        types.StatePeerChanged,
			ChangeNodes: update,
		})
	}

	return &v1.DeleteRouteResponse{}, nil
}

func (api headscaleV1APIServer) CreateApiKey(
	ctx context.Context,
	request *v1.CreateApiKeyRequest,
) (*v1.CreateApiKeyResponse, error) {
	var expiration time.Time
	if request.GetExpiration() != nil {
		expiration = request.GetExpiration().AsTime()
	}

	apiKey, _, err := api.h.db.CreateAPIKey(
		&expiration,
	)
	if err != nil {
		return nil, err
	}

	return &v1.CreateApiKeyResponse{ApiKey: apiKey}, nil
}

func (api headscaleV1APIServer) ExpireApiKey(
	ctx context.Context,
	request *v1.ExpireApiKeyRequest,
) (*v1.ExpireApiKeyResponse, error) {
	var apiKey *types.APIKey
	var err error

	apiKey, err = api.h.db.GetAPIKey(request.Prefix)
	if err != nil {
		return nil, err
	}

	err = api.h.db.ExpireAPIKey(apiKey)
	if err != nil {
		return nil, err
	}

	return &v1.ExpireApiKeyResponse{}, nil
}

func (api headscaleV1APIServer) ListApiKeys(
	ctx context.Context,
	request *v1.ListApiKeysRequest,
) (*v1.ListApiKeysResponse, error) {
	apiKeys, err := api.h.db.ListAPIKeys()
	if err != nil {
		return nil, err
	}

	response := make([]*v1.ApiKey, len(apiKeys))
	for index, key := range apiKeys {
		response[index] = key.Proto()
	}

	sort.Slice(response, func(i, j int) bool {
		return response[i].Id < response[j].Id
	})

	return &v1.ListApiKeysResponse{ApiKeys: response}, nil
}

func (api headscaleV1APIServer) DeleteApiKey(
	ctx context.Context,
	request *v1.DeleteApiKeyRequest,
) (*v1.DeleteApiKeyResponse, error) {
	var (
		apiKey *types.APIKey
		err    error
	)

	apiKey, err = api.h.db.GetAPIKey(request.Prefix)
	if err != nil {
		return nil, err
	}

	if err := api.h.db.DestroyAPIKey(*apiKey); err != nil {
		return nil, err
	}

	return &v1.DeleteApiKeyResponse{}, nil
}

func (api headscaleV1APIServer) GetPolicy(
	_ context.Context,
	_ *v1.GetPolicyRequest,
) (*v1.GetPolicyResponse, error) {
	switch api.h.cfg.Policy.Mode {
	case types.PolicyModeDB:
		p, err := api.h.db.GetPolicy()
		if err != nil {
			return nil, fmt.Errorf("loading ACL from database: %w", err)
		}

		return &v1.GetPolicyResponse{
			Policy:    p.Data,
			UpdatedAt: timestamppb.New(p.UpdatedAt),
		}, nil
	case types.PolicyModeFile:
		// Read the file and return the contents as-is.
		absPath := util.AbsolutePathFromConfigPath(api.h.cfg.Policy.Path)
		f, err := os.Open(absPath)
		if err != nil {
			return nil, fmt.Errorf("reading policy from path %q: %w", absPath, err)
		}

		defer f.Close()

		b, err := io.ReadAll(f)
		if err != nil {
			return nil, fmt.Errorf("reading policy from file: %w", err)
		}

		return &v1.GetPolicyResponse{Policy: string(b)}, nil
	}

	return nil, fmt.Errorf(
		"no supported policy mode found in configuration, policy.mode: %q",
		api.h.cfg.Policy.Mode,
	)
}

func (api headscaleV1APIServer) SetPolicy(
	_ context.Context,
	request *v1.SetPolicyRequest,
) (*v1.SetPolicyResponse, error) {
	if api.h.cfg.Policy.Mode != types.PolicyModeDB {
		return nil, types.ErrPolicyUpdateIsDisabled
	}

	p := request.GetPolicy()

	pol, err := policy.LoadACLPolicyFromBytes([]byte(p))
	if err != nil {
		return nil, fmt.Errorf("loading ACL policy file: %w", err)
	}

	// Validate and reject configuration that would error when applied
	// when creating a map response. This requires nodes, so there is still
	// a scenario where they might be allowed if the server has no nodes
	// yet, but it should help for the general case and for hot reloading
	// configurations.
	nodes, err := api.h.db.ListNodes()
	if err != nil {
		return nil, fmt.Errorf(
			"loading nodes from database to validate policy: %w",
			err,
		)
	}

	_, err = pol.CompileFilterRules(nodes)
	if err != nil {
		return nil, fmt.Errorf("verifying policy rules: %w", err)
	}

	if len(nodes) > 0 {
		_, err = pol.CompileSSHPolicy(nodes[0], nodes)
		if err != nil {
			return nil, fmt.Errorf("verifying SSH rules: %w", err)
		}
	}

	updated, err := api.h.db.SetPolicy(p)
	if err != nil {
		return nil, err
	}

	api.h.ACLPolicy = pol

	api.h.setLastStateChangeToNow()

	ctx := types.NotifyCtx(context.Background(), "acl-update", "na")
	api.h.nodeNotifier.NotifyAll(ctx, types.StateUpdate{
		Type: types.StateFullUpdate,
	})

	response := &v1.SetPolicyResponse{
		Policy:    updated.Data,
		UpdatedAt: timestamppb.New(updated.UpdatedAt),
	}

	return response, nil
}

// --- ACL START ---

// copyACLConfig copies an ACL configuration file from source to destination.
func copyACLConfig(dst, src string) error {
	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	_, err = io.Copy(dstFile, srcFile)

	return err
}

// getPendingACLConfig retrieves the pending ACL configuration.
// It stores the pending ACL config within the directory of the ACL config
// with the file name .acl.json.
func getPendingACLConfig(h *Headscale) (*policy.ACLPolicy, error) {
	inUsedPolicyPath := util.AbsolutePathFromConfigPath(h.cfg.Policy.Path)
	pendingPolicyPath := filepath.Dir(inUsedPolicyPath) + "/.acl.json"
	if _, err := os.Stat(pendingPolicyPath); errors.Is(err, os.ErrNotExist) {
		err = copyACLConfig(pendingPolicyPath, inUsedPolicyPath)
		if err != nil {
			return nil, err
		}
	}

	return policy.LoadACLPolicyFromPath(pendingPolicyPath)
}

// updatePendingACLConfig updates the pending ACL configuration with the provided policy.
func updatePendingACLConfig(h *Headscale, policy *policy.ACLPolicy) error {
	inUsedPolicyPath := util.AbsolutePathFromConfigPath(h.cfg.Policy.Path)
	pendingPolicyPath := filepath.Dir(inUsedPolicyPath) + "/.acl.json"
	pendingFile, err := os.Create(pendingPolicyPath)
	if err != nil {
		return err
	}
	defer pendingFile.Close()

	js := json.NewEncoder(pendingFile)
	return js.Encode(policy)
}

// reloadPendingACLConfig reloads the pending ACL configuration.
func reloadPendingACLConfig(h *Headscale) (*policy.ACLPolicy, error) {
	inUsedPolicyPath := util.AbsolutePathFromConfigPath(h.cfg.Policy.Path)
	pendingPolicyPath := filepath.Dir(inUsedPolicyPath) + "/.acl.json"
	if _, err := os.Stat(pendingPolicyPath); errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	if err := os.Rename(pendingPolicyPath, inUsedPolicyPath); err != nil {
		return nil, err
	}

	h.setLastStateChangeToNow()
	return policy.LoadACLPolicyFromPath(pendingPolicyPath)
}

// discardPendingACLConfig discards the pending ACL configuration.
func discardPendingACLConfig(h *Headscale) error {
	inUsedPolicyPath := util.AbsolutePathFromConfigPath(h.cfg.Policy.Path)
	pendingPolicyPath := filepath.Dir(inUsedPolicyPath) + "/.acl.json"

	return os.Remove(pendingPolicyPath)
}

// ACLCreateGroup creates a new ACL group.
func (api headscaleV1APIServer) ACLCreateGroup(
	_ context.Context,
	request *v1.ACLGroupRequest,
) (*v1.ACLGroupResponse, error) {
	if api.h.cfg.Policy.Mode != types.PolicyModeFile {
		return nil, status.Error(
			codes.FailedPrecondition,
			"ACLAPIs only supported in file mode",
		)
	}
	aclPolicy, err := getPendingACLConfig(api.h)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	groupName := "group:" + request.GetGroupName()

	if aclPolicy.Groups == nil {
		aclPolicy.Groups = make(policy.Groups)
	}

	if _, exists := aclPolicy.Groups[groupName]; exists {
		return nil, status.Error(codes.AlreadyExists, "Group already exists")
	}

	aclPolicy.Groups[groupName] = make([]string, 0)

	if err = updatePendingACLConfig(api.h, aclPolicy); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &v1.ACLGroupResponse{}, nil
}

// ACLGroupAddUser adds a user to an ACL group.
func (api headscaleV1APIServer) ACLGroupAddUser(
	ctx context.Context,
	request *v1.ACLGroupUserRequest,
) (*v1.ACLGroupUserResponse, error) {
	if api.h.cfg.Policy.Mode != types.PolicyModeFile {
		return nil, status.Error(
			codes.FailedPrecondition,
			"ACLAPIs only supported in file mode",
		)
	}

	aclPolicy, err := getPendingACLConfig(api.h)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	groupName := "group:" + request.GetGroupName()
	username := request.GetUsername()

	if aclPolicy.Groups == nil {
		return nil, status.Error(codes.InvalidArgument, "No group exists")
	}

	if _, exists := aclPolicy.Groups[groupName]; !exists {
		return nil, status.Error(codes.InvalidArgument, "Group does not exist")
	}

	if slices.Contains(aclPolicy.Groups[groupName], username) {
		return nil, status.Error(codes.AlreadyExists, "User already in the group")
	}

	aclPolicy.Groups[groupName] = append(aclPolicy.Groups[groupName], username)

	if err = updatePendingACLConfig(api.h, aclPolicy); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &v1.ACLGroupUserResponse{}, nil
}

// ACLGroupRemoveUser removes a user from an ACL group.
func (api headscaleV1APIServer) ACLGroupRemoveUser(
	ctx context.Context,
	request *v1.ACLGroupUserRequest,
) (*v1.ACLGroupUserResponse, error) {
	if api.h.cfg.Policy.Mode != types.PolicyModeFile {
		return nil, status.Error(
			codes.FailedPrecondition,
			"ACLAPIs only supported in file mode",
		)
	}

	aclPolicy, err := getPendingACLConfig(api.h)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	groupName := "group:" + request.GetGroupName()
	username := request.GetUsername()

	if aclPolicy.Groups == nil {
		return nil, status.Error(codes.InvalidArgument, "No group exists")
	}

	if _, exists := aclPolicy.Groups[groupName]; !exists {
		return nil, status.Error(codes.InvalidArgument, "Group does not exist")
	}

	idx := slices.Index(aclPolicy.Groups[groupName], username)

	if idx == -1 {
		return nil, status.Error(codes.InvalidArgument, "User does not in the group")
	}

	aclPolicy.Groups[groupName] = slices.Delete(aclPolicy.Groups[groupName], idx, idx+1)

	if err = updatePendingACLConfig(api.h, aclPolicy); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &v1.ACLGroupUserResponse{}, nil
}

// ACLRemoveGroup removes an ACL group.
func (api headscaleV1APIServer) ACLRemoveGroup(
	ctx context.Context,
	request *v1.ACLGroupRequest,
) (*v1.ACLGroupResponse, error) {
	if api.h.cfg.Policy.Mode != types.PolicyModeFile {
		return nil, status.Error(
			codes.FailedPrecondition,
			"ACLAPIs only supported in file mode",
		)
	}
	aclPolicy, err := getPendingACLConfig(api.h)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	groupName := "group:" + request.GetGroupName()

	if aclPolicy.Groups == nil {
		return nil, status.Error(codes.InvalidArgument, "No group exists")
	}

	if _, exists := aclPolicy.Groups[groupName]; !exists {
		return nil, status.Error(codes.InvalidArgument, "Group does not exist")
	}

	delete(aclPolicy.Groups, groupName)

	if err = updatePendingACLConfig(api.h, aclPolicy); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &v1.ACLGroupResponse{}, nil
}

// ACLBindHostname binds a hostname to a subnet.
func (api headscaleV1APIServer) ACLBindHostname(
	ctx context.Context,
	request *v1.ACLHostnameRequest,
) (*v1.ACLHostnameResponse, error) {
	if api.h.cfg.Policy.Mode != types.PolicyModeFile {
		return nil, status.Error(
			codes.FailedPrecondition,
			"ACLAPIs only supported in file mode",
		)
	}
	aclPolicy, err := getPendingACLConfig(api.h)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	hostname := request.GetHostname()
	hostType := request.GetType()
	address := request.GetAddress()

	if hostType != "subnet" {
		return nil, status.Error(codes.InvalidArgument, "Invalid type")
	}

	hostname = hostType + ":" + hostname

	if aclPolicy.Hosts == nil {
		aclPolicy.Hosts = make(policy.Hosts)
	}

	if _, exists := aclPolicy.Hosts[hostname]; exists {
		return nil, status.Error(codes.AlreadyExists, "Hostname already exists")
	}

	prefix, err := netip.ParsePrefix(address)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	aclPolicy.Hosts[hostname] = prefix

	if err = updatePendingACLConfig(api.h, aclPolicy); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &v1.ACLHostnameResponse{}, nil
}

// ACLUpdateHostname updates a hostname's subnet binding.
func (api headscaleV1APIServer) ACLUpdateHostname(
	ctx context.Context,
	request *v1.ACLHostnameRequest,
) (*v1.ACLHostnameResponse, error) {
	if api.h.cfg.Policy.Mode != types.PolicyModeFile {
		return nil, status.Error(
			codes.FailedPrecondition,
			"ACLAPIs only supported in file mode",
		)
	}
	aclPolicy, err := getPendingACLConfig(api.h)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	hostname := request.GetHostname()
	hostType := request.GetType()
	address := request.GetAddress()

	if hostType != "subnet" {
		return nil, status.Error(codes.InvalidArgument, "Invalid type")
	}

	hostname = hostType + ":" + hostname

	if aclPolicy.Hosts == nil {
		return nil, status.Error(codes.Internal, "No host exists")
	}

	if _, exists := aclPolicy.Hosts[hostname]; !exists {
		return nil, status.Error(codes.InvalidArgument, "Hostname does not exist")
	}

	prefix, err := netip.ParsePrefix(address)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	aclPolicy.Hosts[hostname] = prefix

	if err = updatePendingACLConfig(api.h, aclPolicy); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &v1.ACLHostnameResponse{}, nil
}

// ACLRemoveHostname removes a hostname's subnet binding.
func (api headscaleV1APIServer) ACLRemoveHostname(
	ctx context.Context,
	request *v1.ACLHostnameRequest,
) (*v1.ACLHostnameResponse, error) {
	if api.h.cfg.Policy.Mode != types.PolicyModeFile {
		return nil, status.Error(
			codes.FailedPrecondition,
			"ACLAPIs only supported in file mode",
		)
	}
	aclPolicy, err := getPendingACLConfig(api.h)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	hostname := request.GetHostname()
	hostname = "subnet:" + hostname

	if aclPolicy.Hosts == nil {
		return nil, status.Error(codes.Internal, "No host exists")
	}

	if _, exists := aclPolicy.Hosts[hostname]; !exists {
		return nil, status.Error(codes.InvalidArgument, "Hostname does not exist")
	}

	delete(aclPolicy.Hosts, hostname)

	if err = updatePendingACLConfig(api.h, aclPolicy); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &v1.ACLHostnameResponse{}, nil
}

// ACLCreateTag creates a new ACL tag.
func (api headscaleV1APIServer) ACLCreateTag(
	ctx context.Context,
	request *v1.ACLTagRequest,
) (*v1.ACLTagResponse, error) {
	if api.h.cfg.Policy.Mode != types.PolicyModeFile {
		return nil, status.Error(
			codes.FailedPrecondition,
			"ACLAPIs only supported in file mode",
		)
	}
	aclPolicy, err := getPendingACLConfig(api.h)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	tag := "tag:" + request.GetTag()

	if aclPolicy.TagOwners == nil {
		aclPolicy.TagOwners = make(policy.TagOwners)
	}

	if _, exists := aclPolicy.TagOwners[tag]; exists {
		return nil, status.Error(codes.AlreadyExists, "Tag already exists")
	}

	aclPolicy.TagOwners[tag] = make([]string, 0)

	if err = updatePendingACLConfig(api.h, aclPolicy); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &v1.ACLTagResponse{}, nil
}

// ACLRemoveTag removes an ACL tag.
func (api headscaleV1APIServer) ACLRemoveTag(
	ctx context.Context,
	request *v1.ACLTagRequest,
) (*v1.ACLTagResponse, error) {
	if api.h.cfg.Policy.Mode != types.PolicyModeFile {
		return nil, status.Error(
			codes.FailedPrecondition,
			"ACLAPIs only supported in file mode",
		)
	}
	aclPolicy, err := getPendingACLConfig(api.h)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	tag := "tag:" + request.GetTag()

	if aclPolicy.TagOwners == nil {
		return nil, status.Error(codes.InvalidArgument, "No tag exists")
	}

	if _, exists := aclPolicy.TagOwners[tag]; !exists {
		return nil, status.Error(codes.InvalidArgument, "Tag does not exist")
	}

	delete(aclPolicy.TagOwners, tag)

	if err = updatePendingACLConfig(api.h, aclPolicy); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &v1.ACLTagResponse{}, nil
}

// getACLRuleIdx returns the index of an ACL rule in a list of rules,
// or -1 if the rule is not found.
func getACLRuleIdx(rules []policy.ACL, target policy.ACL) int {
	slices.Sort(target.Sources)
	slices.Sort(target.Destinations)

	for i, rule := range rules {
		if rule.Action != target.Action || rule.Protocol != target.Protocol {
			continue
		}

		slices.Sort(rule.Sources)
		if !slices.Equal(rule.Sources, target.Sources) {
			continue
		}

		slices.Sort(rule.Destinations)
		if !slices.Equal(rule.Destinations, target.Destinations) {
			continue
		}

		return i
	}

	return -1
}

// getACLRuleIdxBySrc returns the index of an ACL rule in a list of rules,
// matching only the source, or -1 if the rule is not found.
func getACLRuleIdxBySrc(rules []policy.ACL, target policy.ACL) int {
	slices.Sort(target.Sources)

	for i, rule := range rules {
		if rule.Action != target.Action || rule.Protocol != target.Protocol {
			continue
		}

		slices.Sort(rule.Sources)
		if !slices.Equal(rule.Sources, target.Sources) {
			continue
		}

		return i
	}

	return -1
}

// getACLRuleIdxsByDst returns the indices of ACL rules in a list of rules,
// matching the destinations, or an empty slice if no rule is found.
func getACLRuleIdxsByDst(rules []policy.ACL, target policy.ACL) []int {
	slices.Sort(target.Destinations)
	idxs := []int{}

	for i, rule := range rules {
		if rule.Action != target.Action || rule.Protocol != target.Protocol {
			continue
		}
		slices.Sort(rule.Destinations)
		for _, dest := range target.Destinations {
			if slices.Contains(rule.Destinations, dest) {
				idxs = append(idxs, i)
				break
			}
		}
	}

	return idxs
}

// getACLDstIdx returns the index of a destination in a list of destinations,
// or -1 if the destination is not found.
func getACLDstIdx(ruleDsts []string, targetDst string) int {
	slices.Sort(ruleDsts)

	for i, ruleDst := range ruleDsts {
		if ruleDst == targetDst {
			return i
		}
	}

	return -1
}

// ACLCreateRule creates a new ACL rule.
func (api headscaleV1APIServer) ACLCreateRule(
	ctx context.Context,
	request *v1.ACLRuleRequest,
) (*v1.ACLRuleResponse, error) {
	if api.h.cfg.Policy.Mode != types.PolicyModeFile {
		return nil, status.Error(
			codes.FailedPrecondition,
			"ACLAPIs only supported in file mode",
		)
	}
	aclPolicy, err := getPendingACLConfig(api.h)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	newRule := policy.ACL{
		Action:       "accept",
		Protocol:     "",
		Sources:      request.GetSrc(),
		Destinations: request.GetDst(),
	}

	if newRule.Sources == nil || len(newRule.Sources) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Source should not be empty")
	}

	if newRule.Destinations == nil || len(newRule.Destinations) == 0 {
		return nil, status.Error(
			codes.InvalidArgument,
			"Destination should not be empty",
		)
	}

	if idx := getACLRuleIdx(aclPolicy.ACLs, newRule); idx != -1 {
		return nil, status.Error(codes.AlreadyExists, "Rule exists")
	}

	aclPolicy.ACLs = append(aclPolicy.ACLs, newRule)

	if err = updatePendingACLConfig(api.h, aclPolicy); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &v1.ACLRuleResponse{}, nil
}

func (api headscaleV1APIServer) ACLRemoveRule(
	ctx context.Context,
	request *v1.ACLRuleRequest,
) (*v1.ACLRuleResponse, error) {
	if api.h.cfg.Policy.Mode != types.PolicyModeFile {
		return nil, status.Error(
			codes.FailedPrecondition,
			"ACLAPIs only supported in file mode",
		)
	}
	aclPolicy, err := getPendingACLConfig(api.h)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	targetRule := policy.ACL{
		Action:       "accept",
		Protocol:     "",
		Sources:      request.GetSrc(),
		Destinations: request.GetDst(),
	}

	idx := getACLRuleIdx(aclPolicy.ACLs, targetRule)
	if idx == -1 {
		return nil, status.Error(codes.InvalidArgument, "Rule does not exists")
	}

	aclPolicy.ACLs = slices.Delete(aclPolicy.ACLs, idx, idx+1)

	if err = updatePendingACLConfig(api.h, aclPolicy); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &v1.ACLRuleResponse{}, nil
}

func (api headscaleV1APIServer) ACLForceRemoveRule(
	ctx context.Context,
	request *v1.ACLRuleRequest,
) (*v1.ACLRuleResponse, error) {
	if api.h.cfg.Policy.Mode != types.PolicyModeFile {
		return nil, status.Error(
			codes.FailedPrecondition,
			"ACLAPIs only supported in file mode",
		)
	}
	aclPolicy, err := getPendingACLConfig(api.h)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	targetRule := policy.ACL{
		Action:   "accept",
		Protocol: "",
		Sources:  request.GetSrc(),
	}

	idx := getACLRuleIdxBySrc(aclPolicy.ACLs, targetRule)
	if idx == -1 {
		// force remove, return OK even rule not exist
		return &v1.ACLRuleResponse{}, nil
	}

	aclPolicy.ACLs = slices.Delete(aclPolicy.ACLs, idx, idx+1)

	if err = updatePendingACLConfig(api.h, aclPolicy); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &v1.ACLRuleResponse{}, nil
}

func includeString(source []string, target []string) []string {
	sourceMap := make(map[string]bool)
	for _, s := range source {
		sourceMap[s] = true
	}

	for _, t := range target {
		if !sourceMap[t] {
			source = append(source, t)
		}
	}
	return source
}

func excludeString(source []string, target []string) []string {
	targetMap := make(map[string]bool)
	for _, t := range target {
		targetMap[t] = true
	}

	var result []string
	for _, s := range source {
		if !targetMap[s] {
			result = append(result, s)
		}
	}
	return result
}

func (api headscaleV1APIServer) ACLRuleInclude(
	ctx context.Context,
	request *v1.ACLRuleRequest,
) (*v1.ACLRuleResponse, error) {
	if api.h.cfg.Policy.Mode != types.PolicyModeFile {
		return nil, status.Error(
			codes.FailedPrecondition,
			"ACLAPIs only supported in file mode",
		)
	}
	aclPolicy, err := getPendingACLConfig(api.h)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	targetRule := policy.ACL{
		Action:       "accept",
		Protocol:     "",
		Sources:      request.GetSrc(),
		Destinations: request.GetDst(),
	}

	idx := getACLRuleIdxBySrc(aclPolicy.ACLs, targetRule)
	if idx == -1 {
		if targetRule.Sources == nil || len(targetRule.Sources) == 0 {
			return nil, status.Error(
				codes.InvalidArgument,
				"Source should not be empty",
			)
		}
		if targetRule.Destinations == nil || len(targetRule.Destinations) == 0 {
			return nil, status.Error(
				codes.InvalidArgument,
				"Destination should not be empty",
			)
		}
		aclPolicy.ACLs = append(aclPolicy.ACLs, targetRule)
	} else {
		aclPolicy.ACLs[idx].Destinations = includeString(aclPolicy.ACLs[idx].Destinations, targetRule.Destinations)
	}

	if err = updatePendingACLConfig(api.h, aclPolicy); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &v1.ACLRuleResponse{}, nil
}

func (api headscaleV1APIServer) ACLRuleExclude(
	ctx context.Context,
	request *v1.ACLRuleRequest,
) (*v1.ACLRuleResponse, error) {
	if api.h.cfg.Policy.Mode != types.PolicyModeFile {
		return nil, status.Error(
			codes.FailedPrecondition,
			"ACLAPIs only supported in file mode",
		)
	}
	aclPolicy, err := getPendingACLConfig(api.h)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	newDests := []string{}

	if len(request.GetSrc()) != 0 {
		targetRule := policy.ACL{
			Action:       "accept",
			Protocol:     "",
			Sources:      request.GetSrc(),
			Destinations: request.GetDst(),
		}

		idx := getACLRuleIdxBySrc(aclPolicy.ACLs, targetRule)
		if idx == -1 {
			return nil, status.Error(codes.InvalidArgument, "Rule does not exists")
		}

		newDests = excludeString(
			aclPolicy.ACLs[idx].Destinations,
			targetRule.Destinations,
		)

		if len(newDests) == 0 {
			aclPolicy.ACLs = slices.Delete(aclPolicy.ACLs, idx, idx+1)
		} else {
			aclPolicy.ACLs[idx].Destinations = newDests
		}

	} else {
		targetRule := policy.ACL{
			Action:       "accept",
			Protocol:     "",
			Destinations: request.GetDst(),
		}

		idxs := getACLRuleIdxsByDst(aclPolicy.ACLs, targetRule)
		if len(idxs) == 0 {
			return nil, status.Error(codes.InvalidArgument, "Rule not found for exclude Destinations")
		}

		for i := len(idxs) - 1; i >= 0; i-- {
			idx := idxs[i]
			newDests = excludeString(aclPolicy.ACLs[idx].Destinations, targetRule.Destinations)

			if len(newDests) == 0 {
				aclPolicy.ACLs = slices.Delete(aclPolicy.ACLs, idx, idx+1)
			} else {
				aclPolicy.ACLs[idx].Destinations = newDests
			}
		}
	}

	if err = updatePendingACLConfig(api.h, aclPolicy); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &v1.ACLRuleResponse{}, nil
}

func (api headscaleV1APIServer) ACLCtrl(
	ctx context.Context,
	request *v1.ACLCtrlRequest,
) (*v1.ACLCtrlResponse, error) {
	if api.h.cfg.Policy.Mode != types.PolicyModeFile {
		return nil, status.Error(
			codes.FailedPrecondition,
			"ACLAPIs only supported in file mode",
		)
	}
	action := request.GetAction()

	switch action {
	case "reload":
		reloadPendingACLConfig(api.h)
	case "discard":
		discardPendingACLConfig(api.h)
	default:
		return nil, status.Error(codes.InvalidArgument, "Not supported action")
	}

	return &v1.ACLCtrlResponse{}, nil
}

// --- ACL END ---

// The following service calls are for testing and debugging
func (api headscaleV1APIServer) DebugCreateNode(
	ctx context.Context,
	request *v1.DebugCreateNodeRequest,
) (*v1.DebugCreateNodeResponse, error) {
	user, err := api.h.db.GetUser(request.GetUser())
	if err != nil {
		return nil, err
	}

	routes, err := util.StringToIPPrefix(request.GetRoutes())
	if err != nil {
		return nil, err
	}

	log.Trace().
		Caller().
		Interface("route-prefix", routes).
		Interface("route-str", request.GetRoutes()).
		Msg("")

	hostinfo := tailcfg.Hostinfo{
		RoutableIPs: routes,
		OS:          "TestOS",
		Hostname:    "DebugTestNode",
	}

	var mkey key.MachinePublic
	err = mkey.UnmarshalText([]byte(request.GetKey()))
	if err != nil {
		return nil, err
	}

	nodeKey := key.NewNode()

	newNode := types.Node{
		MachineKey: mkey,
		NodeKey:    nodeKey.Public(),
		Hostname:   request.GetName(),
		User:       *user,

		Expiry:   &time.Time{},
		LastSeen: &time.Time{},

		Hostinfo: &hostinfo,
	}

	log.Debug().
		Str("machine_key", mkey.ShortString()).
		Msg("adding debug machine via CLI, appending to registration cache")

	api.h.registrationCache.Set(
		mkey.String(),
		newNode,
		registerCacheExpiration,
	)

	return &v1.DebugCreateNodeResponse{Node: newNode.Proto()}, nil
}

func (api headscaleV1APIServer) mustEmbedUnimplementedHeadscaleServiceServer() {}
