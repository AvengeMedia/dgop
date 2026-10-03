package gops

import (
	"context"
	"testing"
)

func TestGetMetaProcessNetModifier(t *testing.T) {
	if _, err := dumpSockets(); err != nil {
		t.Skip("sock_diag unavailable")
	}
	g := NewGopsUtil()
	ctx := context.Background()
	for _, mods := range [][]string{{"process-net"}, {"processes", "process-net"}, {"all", "process-net"}} {
		meta, err := g.GetMeta(ctx, mods, MetaParams{ProcLimit: 0, EnableCPU: true})
		if err != nil {
			t.Fatalf("%v: %v", mods, err)
		}
		if row := ownRow(meta.Processes); row == nil || row.Connections == nil {
			t.Errorf("%v: own PID has no connections", mods)
		}
	}

	meta, err := g.GetMeta(ctx, []string{"processes"}, MetaParams{ProcLimit: 0, EnableCPU: true})
	if err != nil {
		t.Fatal(err)
	}
	if row := ownRow(meta.Processes); row == nil || row.Connections != nil {
		t.Errorf("processes without process-net: row=%v, want present with nil Connections", row)
	}
}
