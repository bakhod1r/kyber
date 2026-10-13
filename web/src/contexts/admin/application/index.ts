// admin: application layer — the use cases the UI calls. They run through the gateway; the UI never
// imports infrastructure directly, so the transport can change without touching components.
import { adminGateway } from "../infrastructure/gateway";

export const adminUseCases = adminGateway;
export * from "../domain/model";
