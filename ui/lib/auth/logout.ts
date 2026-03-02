import { clearTokens } from "./tokens";

export function logout(): void {
    clearTokens();
    window.location.href = '/login';
}