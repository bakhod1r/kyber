// insights: application layer — the use cases the UI calls. They run through the gateway; the UI never
// imports infrastructure directly, so the transport can change without touching components.
import { insightsGateway } from "../infrastructure/gateway";

export const insightsUseCases = insightsGateway;
export * from "../domain/model";
