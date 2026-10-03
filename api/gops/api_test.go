package gops_handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	gops_handler "github.com/AvengeMedia/dgop/api/gops"
	"github.com/AvengeMedia/dgop/api/server"
	"github.com/AvengeMedia/dgop/gops"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"
)

func newAPI(t *testing.T) humatest.TestAPI {
	_, api := humatest.New(t)
	gops_handler.RegisterHandlers(&server.Server{Gops: gops.NewGopsUtil()}, huma.NewGroup(api, "/gops"))
	return api
}

func TestNewEndpoints(t *testing.T) {
	api := newAPI(t)
	cases := []struct{ path, key string }{
		{"/gops/sensors", "sensors"},
		{"/gops/gpu/stats", "gpus"},
		{"/gops/memory/modules", "modules"},
	}
	for _, c := range cases {
		resp := api.Get(c.path)
		if resp.Code != http.StatusOK {
			t.Fatalf("%s: status %d: %s", c.path, resp.Code, resp.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s: %v", c.path, err)
		}
		if _, ok := body[c.key]; !ok {
			t.Errorf("%s: missing key %q in %s", c.path, c.key, resp.Body.String())
		}
	}
}

func TestNetRateAllInterfaces(t *testing.T) {
	if resp := newAPI(t).Get("/gops/net-rate?all_interfaces=true"); resp.Code != http.StatusOK {
		t.Fatalf("status %d: %s", resp.Code, resp.Body.String())
	}
}

func TestMetaNewParams(t *testing.T) {
	resp := newAPI(t).Get("/gops/meta?modules=sensors,gpu-stats,memory-modules&gpu_stats_cursor=&net_rate_all_interfaces=true")
	if resp.Code != http.StatusOK {
		t.Fatalf("status %d: %s", resp.Code, resp.Body.String())
	}
}

func TestOpenAPIListsNewOperations(t *testing.T) {
	api := newAPI(t)
	for _, p := range []string{"/gops/sensors", "/gops/gpu/stats", "/gops/memory/modules"} {
		if api.OpenAPI().Paths[p] == nil {
			t.Errorf("%s missing from OpenAPI", p)
		}
	}
}

// An absent gpu_pci_ids means all GPUs; a present one, even empty, is an allow-list.
func TestGPUPciIdsQueryDistinguishesEmpty(t *testing.T) {
	_, api := humatest.New(t)
	var got []string
	huma.Get(api, "/x", func(_ context.Context, in *gops_handler.GPUStatsInput) (*struct{}, error) {
		got = in.GPUPciIds
		return nil, nil
	})
	api.Get("/x")
	if got != nil {
		t.Errorf("absent: got %#v, want nil", got)
	}
	api.Get("/x?gpu_pci_ids=")
	if got == nil {
		t.Errorf("empty: got nil, want non-nil empty list")
	}
	api.Get("/x?gpu_pci_ids=10de:2684&gpu_pci_ids=1002:164e")
	if len(got) != 2 || got[1] != "1002:164e" {
		t.Errorf("repeated: got %#v", got)
	}
	api.Get("/x?gpu_pci_ids=10de:2684,1002:164e")
	if len(got) != 2 || got[1] != "1002:164e" {
		t.Errorf("comma: got %#v", got)
	}
}

func TestResolveGPUPciIds(t *testing.T) {
	_, api := humatest.New(t)
	var meta []string
	huma.Get(api, "/m", func(_ context.Context, in *gops_handler.MetaInput) (*struct{}, error) {
		meta = in.GPUPciIds
		return nil, nil
	})
	api.Get("/m?modules=x&gpu_pci_ids=")
	if meta == nil {
		t.Errorf("empty must be non-nil: meta=%#v", meta)
	}
	api.Get("/m?modules=x&gpu_pci_ids=a:b,c:d")
	if len(meta) != 2 {
		t.Errorf("meta=%#v", meta)
	}
	api.Get("/m?modules=x")
	if meta != nil {
		t.Errorf("absent must be nil: meta=%#v", meta)
	}
}
