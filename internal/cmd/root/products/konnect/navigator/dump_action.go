package navigator

import (
	"fmt"
	"reflect"
	"strings"
	"unicode"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs/dump"
)

var dumpResourceByViewParent = map[string]string{
	common.ViewParentAPI:                "apis",
	common.ViewParentAnalyticsDashboard: "analytics.dashboards",
	common.ViewParentAuthStrategy:       "application_auth_strategies",
	common.ViewParentControlPlane:       "control_planes",
	common.ViewParentDCRProvider:        "dcr_providers",
	common.ViewParentEventGateway:       "event_gateways",
	common.ViewParentPortal:             "portals",
	common.ViewParentTeam:               "organization.teams",
}

var dumpResourceByNavigatorLabel = map[string]string{
	common.ViewResourceAPIs:           "apis",
	common.ViewResourceAuthStrategies: "application_auth_strategies",
	common.ViewResourceControlPlanes:  "control_planes",
	common.ViewResourceDCRProviders:   "dcr_providers",
	common.ViewResourceEventGateways:  "event_gateways",
	common.ViewResourcePortals:        "portals",
}

func newDumpSelectionAction(helper cmdpkg.Helper) tableview.SelectionAction {
	return tableview.SelectionAction{
		Key:  "d",
		Help: "dump selected resource",
		Resolve: func(selection tableview.SelectionContext) (tableview.SelectionActionCommand, error) {
			resource, err := dumpResourceForSelection(selection)
			if err != nil {
				return tableview.SelectionActionCommand{}, err
			}

			resourceID := stringField(selection.Parent, "ID", "Id")
			if selection.Parent != nil && resourceID == "" {
				return tableview.SelectionActionCommand{}, fmt.Errorf("selected resource has no resolvable id")
			}

			label := strings.TrimSpace(selection.Label)
			if label == "" {
				label = resource
			}
			title := fmt.Sprintf("Dump %s", label)
			return tableview.SelectionActionCommand{
				Title:               title,
				Label:               title,
				DefaultOutputFile:   defaultDumpOutputFile(resource, label, resourceID),
				IncludeChildrenText: "include child resources",
				Run: func(values tableview.SelectionActionValues) error {
					return dump.RunDeclarativeDump(helper, dump.DeclarativeDumpOptions{
						Resources:             []string{resource},
						OutputFile:            values.OutputFile,
						DefaultNamespace:      values.DefaultNamespace,
						IncludeChildResources: values.IncludeChildren,
						FilterID:              resourceID,
					})
				},
			}, nil
		},
	}
}

func dumpResourceForSelection(selection tableview.SelectionContext) (string, error) {
	parentType := strings.ToLower(strings.TrimSpace(selection.ParentType))
	if resource, ok := dumpResourceByViewParent[parentType]; ok {
		return resource, nil
	}

	for _, candidate := range selectionCandidates(selection) {
		key := strings.ToLower(strings.TrimSpace(candidate))
		if resource, ok := dumpResourceByNavigatorLabel[key]; ok {
			return resource, nil
		}
	}

	label := strings.TrimSpace(selection.Label)
	if label == "" {
		label = strings.TrimSpace(selection.ParentType)
	}
	if label == "" {
		label = "selection"
	}
	return "", fmt.Errorf("dump is not available for %s", label)
}

func selectionCandidates(selection tableview.SelectionContext) []string {
	candidates := []string{selection.Label}
	candidates = append(candidates, selection.Row...)
	return candidates
}

func defaultDumpOutputFile(resource, label, id string) string {
	resourceSlug := filenameSlug(resource)
	if resourceSlug == "" {
		resourceSlug = "resource"
	}

	targetSlug := filenameSlug(label)
	if strings.TrimSpace(id) == "" {
		return resourceSlug + ".yaml"
	}
	if targetSlug == "" || targetSlug == resourceSlug {
		targetSlug = filenameSlug(shortResourceID(id))
	}
	if targetSlug == "" {
		return resourceSlug + ".yaml"
	}
	return resourceSlug + "-" + targetSlug + ".yaml"
}

func filenameSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteRune('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func shortResourceID(id string) string {
	id = strings.TrimSpace(id)
	if len(id) <= 12 {
		return id
	}
	return id[:12]
}

func stringField(source any, fieldNames ...string) string {
	value := reflect.ValueOf(source)
	for value.IsValid() && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			return ""
		}
		value = value.Elem()
	}
	if !value.IsValid() || value.Kind() != reflect.Struct {
		return ""
	}

	for _, name := range fieldNames {
		field := value.FieldByName(name)
		if result := stringValue(field); result != "" {
			return result
		}
	}
	return ""
}

func stringValue(value reflect.Value) string {
	for value.IsValid() && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			return ""
		}
		value = value.Elem()
	}
	if !value.IsValid() || value.Kind() != reflect.String {
		return ""
	}
	return strings.TrimSpace(value.String())
}
