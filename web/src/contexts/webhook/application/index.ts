// webhook: application layer — the use cases the UI calls, through the gateway.
import { webhookGateway } from "../infrastructure/gateway";

export const webhookUseCases = webhookGateway;
export * from "../domain/model";
