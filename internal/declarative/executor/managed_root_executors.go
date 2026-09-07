package executor

func (e *Executor) registerAuthStrategyExecutor() {
	e.registerResourceExecutor(prepareResourceWrites(crudResourceExecutor(
		NewManagedLabelBaseExecutor(NewAuthStrategyAdapter(e.client), e.client, e.dryRun),
	), e.syncResolvedDCRProviderID))
}

func (e *Executor) registerDCRProviderExecutor() {
	e.registerResourceExecutor(crudResourceExecutor(
		NewManagedLabelBaseExecutor(NewDCRProviderAdapter(e.client), e.client, e.dryRun),
	))
}

func (e *Executor) registerCatalogServiceExecutor() {
	e.registerResourceExecutor(crudResourceExecutor(
		NewManagedLabelBaseExecutor(NewCatalogServiceAdapter(e.client), e.client, e.dryRun),
	))
}

func (e *Executor) registerDashboardExecutor() {
	e.registerResourceExecutor(crudResourceExecutor(
		NewManagedLabelBaseExecutor(NewDashboardAdapter(e.client), e.client, e.dryRun),
	))
}
