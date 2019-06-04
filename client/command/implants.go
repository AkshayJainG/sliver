package command

import (
	"bytes"
	"fmt"
	"strings"
	"text/tabwriter"

	clientpb "github.com/bishopfox/sliver/protobuf/client"
	implantpb "github.com/bishopfox/sliver/protobuf/implant"

	"github.com/desertbit/grumble"
	"github.com/golang/protobuf/proto"
)

// `slivers` command impl
func listSliverBuilds(ctx *grumble.Context, rpc RPCServer) {

	resp := <-rpc(&implantpb.Envelope{
		Type: clientpb.MsgListSliverBuilds,
	}, defaultTimeout)
	if resp.Err != "" {
		fmt.Printf(Warn+"%s\n", resp.Err)
		return
	}

	builds := &clientpb.SliverBuilds{}
	proto.Unmarshal(resp.Data, builds)
	if 0 < len(builds.Configs) {
		displayAllSliverBuilds(builds.Configs)
	} else {
		fmt.Printf(Info + "No sliver builds\n")
	}
}

func displayAllSliverBuilds(configs map[string]*clientpb.SliverConfig) {

	outputBuf := bytes.NewBufferString("")
	table := tabwriter.NewWriter(outputBuf, 0, 2, 2, ' ', 0)

	fmt.Fprintf(table, "Name\tOS/Arch\tDebug\tFormat\t\n")
	fmt.Fprintf(table, "%s\t%s\t%s\t%s\t\n",
		strings.Repeat("=", len("Name")),
		strings.Repeat("=", len("OS/Arch")),
		strings.Repeat("=", len("Debug")),
		strings.Repeat("=", len("Format")),
	)

	for sliverName, config := range configs {
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t\n",
			sliverName,
			fmt.Sprintf("%s/%s", config.GOOS, config.GOARCH),
			fmt.Sprintf("%v", config.Debug),
			config.Format,
		)
	}
	table.Flush()
	fmt.Printf(outputBuf.String())
}
