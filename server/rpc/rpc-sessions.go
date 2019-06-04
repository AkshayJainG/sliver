package rpc

import (
	"time"

	clientpb "github.com/bishopfox/sliver/protobuf/client"
	implantpb "github.com/bishopfox/sliver/protobuf/implant"
	"github.com/bishopfox/sliver/server/core"

	"github.com/golang/protobuf/proto"
)

func rpcKill(data []byte, timeout time.Duration, resp RPCResponse) {
	killReq := &implantpb.KillReq{}
	err := proto.Unmarshal(data, killReq)
	if err != nil {
		resp([]byte{}, err)
	}
	sliver := core.Hive.Sliver(killReq.SliverID)
	data, err = sliver.Request(implantpb.MsgKill, timeout, data)
	core.Hive.RemoveSliver(sliver)
	resp(data, err)
}

func rpcSessions(_ []byte, timeout time.Duration, resp RPCResponse) {
	sessions := &clientpb.Sessions{}
	if 0 < len(*core.Hive.Slivers) {
		for _, sliver := range *core.Hive.Slivers {
			sessions.Slivers = append(sessions.Slivers, sliver.ToProtobuf())
		}
	}
	data, err := proto.Marshal(sessions)
	if err != nil {
		rpcLog.Errorf("Error encoding rpc response %v", err)
	}
	resp(data, err)
}
