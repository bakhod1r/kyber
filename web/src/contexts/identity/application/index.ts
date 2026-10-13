// identity: application layer — the use cases the UI calls. They run through the gateway; the UI never
// imports infrastructure directly, so the transport can change without touching components.
import { identityGateway } from "../infrastructure/gateway";

export const identityUseCases = identityGateway;
export * from "../domain/model";
