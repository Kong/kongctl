package executor

// registerAIGatewayExecutors is the runtime inventory for AI Gateway resources.
// Each entry supplies construction, supported actions, and payload validation.
func (e *Executor) registerAIGatewayExecutors() {
	client, dryRun := e.client, e.dryRun
	e.registerResourceExecutor(crudResourceExecutor(
		NewManagedLabelBaseExecutor(NewAIGatewayAdapter(client), client, dryRun),
	))

	registerChild := func(resource resourceExecutor) {
		e.registerResourceExecutor(prepareResourceExecutor(resource, e.syncResolvedAIGatewayID))
	}
	registerChild(crudResourceExecutor(
		NewBaseExecutor(NewAIGatewayProviderAdapter(client), client, dryRun),
	))
	registerChild(crudResourceExecutor(
		NewBaseExecutor(NewAIGatewayAuthStrategyAdapter(client), client, dryRun),
	))
	registerChild(crudResourceExecutor(
		NewBaseExecutor(NewAIGatewayPolicyAdapter(client), client, dryRun),
	))
	registerChild(crudResourceExecutor(
		NewBaseExecutor(NewAIGatewayAgentAdapter(client), client, dryRun),
	))
	registerChild(crudResourceExecutor(
		NewBaseExecutor(NewAIGatewayConsumerAdapter(client), client, dryRun),
	))
	registerChild(createDeleteResourceExecutor(
		NewBaseCreateDeleteExecutor(NewAIGatewayConsumerCredentialAdapter(client), dryRun),
	))

	consumerGroup := crudResourceExecutor(
		NewBaseExecutor(NewAIGatewayConsumerGroupAdapter(client), client, dryRun),
	)
	consumerGroup.create = afterResourceWrite(consumerGroup.create, e.syncAIGatewayConsumerGroupConsumers)
	consumerGroup.update = afterResourceWrite(consumerGroup.update, e.syncAIGatewayConsumerGroupConsumers)
	registerChild(consumerGroup)

	registerChild(crudResourceExecutor(
		NewBaseExecutor(NewAIGatewayModelAdapter(client), client, dryRun),
	))
	registerChild(crudResourceExecutor(
		NewBaseExecutor(NewAIGatewayMCPServerAdapter(client), client, dryRun),
	))
	registerChild(crudResourceExecutor(
		NewBaseExecutor(NewAIGatewayVaultAdapter(client), client, dryRun),
	))
	registerChild(crudResourceExecutor(
		NewBaseExecutor(NewAIGatewayConfigStoreAdapter(client), client, dryRun),
	))
	registerChild(crudResourceExecutor(
		NewBaseExecutor(NewAIGatewayConfigStoreSecretAdapter(client), client, dryRun),
	))
	registerChild(createDeleteResourceExecutor(
		NewBaseCreateDeleteExecutor(NewAIGatewayDataPlaneCertificateAdapter(client), dryRun),
	))
	registerChild(crudResourceExecutor(
		NewBaseExecutor(NewAIGatewayCertificateAdapter(client), client, dryRun),
	))
	registerChild(crudResourceExecutor(
		NewBaseExecutor(NewAIGatewayCACertificateAdapter(client), client, dryRun),
	))
	registerChild(crudResourceExecutor(
		NewBaseExecutor(NewAIGatewaySNIAdapter(client), client, dryRun),
	))
}
