// calendar: application layer — the use cases the UI calls. They run through the gateway; the UI never
// imports infrastructure directly, so the transport can change without touching components.
import { calendarGateway } from "../infrastructure/gateway";

export const calendarUseCases = calendarGateway;
export * from "../domain/model";
