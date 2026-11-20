package resources

import "reflect"

var aiCostExplainCollections = map[ResourceType]string{
	ResourceTypeAICostBudget:   "budgets",
	ResourceTypeAICostChannel:  "notification_channels",
	ResourceTypeAICostSchedule: "export_schedules",
	ResourceTypeAICostRule:     "notification_rules",
}

func aiCostExplainCollection(kind ResourceType) string { return aiCostExplainCollections[kind] }

func aiCostExplainKind(collection string) (ResourceType, bool) {
	for kind, key := range aiCostExplainCollections {
		if key == collection {
			return kind, true
		}
	}
	return "", false
}

func applyAICostScaffold(subject *ExplainSubject, collection string) error {
	root, err := autoExplainNode(reflect.TypeFor[AIResource](), nil, defaultExplainHints(""), nil)
	if err != nil {
		return err
	}
	cost, _ := root.property("cost_management")
	subject.ScaffoldSteps = []ExplainScaffoldStep{{Name: "ai"}, {Name: "cost_management"}, {Name: collection, Array: true}}
	subject.ScaffoldTrail = []ExplainScaffoldNode{
		{Step: subject.ScaffoldSteps[0], Node: root},
		{Step: subject.ScaffoldSteps[1], Node: cost.Node},
		{Step: subject.ScaffoldSteps[2], Node: subject.Doc.Schema.clone()},
	}
	return nil
}
