package navigator

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/table"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/products"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

const homeLabel = "Konnect"

// Options configures the Konnect resource navigator home view.
type Options struct {
	InitialResource string
}

type resourceLoader func(cmd.Helper) (tableview.ChildView, error)

type resource struct {
	label   string
	display string
	aliases []string
	load    resourceLoader
}

type resourceRecord struct {
	Name string
}

var (
	resourceMu sync.RWMutex
	resources  []resource
)

// RegisterResource registers a resource loader with the navigator. Call this from an init
// function in the resource package to make it available in the home view.
func RegisterResource(label string, aliases []string, loader func(cmd.Helper) (tableview.ChildView, error)) {
	resourceMu.Lock()
	defer resourceMu.Unlock()

	normalized := append([]string(nil), aliases...)
	if len(normalized) == 0 {
		normalized = []string{label}
	} else {
		for i := range normalized {
			normalized[i] = strings.TrimSpace(normalized[i])
		}
	}

	display := strings.TrimSpace(label)
	if display == "" {
		display = label
	}

	resources = append(resources, resource{
		label:   strings.TrimSpace(label),
		display: display,
		aliases: normalized,
		load:    loader,
	})
}

// Run renders the Konnect resource navigator using the registered resources.
func Run(helper cmd.Helper, opts Options) error {
	cmd := helper.GetCmd()
	if cmd != nil {
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		if ctx.Value(products.Product) == nil {
			ctx = context.WithValue(ctx, products.Product, products.ProductValue("konnect"))
		}
		if ctx.Value(helpers.SDKAPIFactoryKey) == nil {
			ctx = context.WithValue(ctx, helpers.SDKAPIFactoryKey, common.GetSDKFactory())
		}
		cmd.SetContext(ctx)
	}

	ensureRequestPageSize(helper)
	profileName := ""
	if cfg, err := helper.GetConfig(); err == nil && cfg != nil {
		profileName = cfg.GetProfile()
	}

	resourceMu.RLock()
	if len(resources) == 0 {
		resourceMu.RUnlock()
		return fmt.Errorf("konnect navigator: no resources registered")
	}
	entries := make([]resource, len(resources))
	copy(entries, resources)
	resourceMu.RUnlock()

	sort.Slice(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].label) < strings.ToLower(entries[j].label)
	})

	records := make([]resourceRecord, len(entries))
	rows := make([]table.Row, len(entries))
	initialIndex := -1
	needle := strings.TrimSpace(strings.ToLower(opts.InitialResource))

	for i, res := range entries {
		records[i] = resourceRecord{Name: res.label}
		rows[i] = table.Row{res.label}
		if needle == "" {
			continue
		}
		for _, alias := range res.aliases {
			if strings.ToLower(alias) == needle {
				initialIndex = i
				break
			}
		}
	}

	loader := func(index int) (tableview.ChildView, error) {
		if index < 0 || index >= len(entries) {
			return tableview.ChildView{}, fmt.Errorf("invalid selection")
		}

		childView, err := entries[index].load(helper)
		if err != nil {
			return tableview.ChildView{}, err
		}

		childView.Title = entries[index].display
		return childView, nil
	}

	options := []tableview.Option{
		tableview.WithCustomTable([]string{"RESOURCE"}, rows),
		tableview.WithRootLabel(homeLabel),
		tableview.WithRowLoader(loader),
		tableview.WithDetailHelper(helper),
		tableview.WithTitle("Konnect Resources"),
		tableview.WithTableStretch(),
	}
	if initialIndex >= 0 {
		options = append(options, tableview.WithInitialRowSelection(initialIndex, needle != ""))
	}
	if profileName != "" {
		options = append(options, tableview.WithProfileName(profileName))
	}

	return tableview.Render(helper.GetStreams(), records, options...)
}

func ensureRequestPageSize(helper cmd.Helper) {
	cfg, err := helper.GetConfig()
	if err != nil {
		return
	}
	if cfg.GetInt(common.RequestPageSizeConfigPath) <= 0 {
		cfg.Set(common.RequestPageSizeConfigPath, common.DefaultRequestPageSize)
	}
}
