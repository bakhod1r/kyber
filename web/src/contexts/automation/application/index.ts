// automation: application layer — the use cases the UI calls, through the gateway.
import { automationGateway } from "../infrastructure/gateway";

export const automationUseCases = automationGateway;
export * from "../domain/model";
