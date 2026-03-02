export const AUTH_CLIENT_ID = 'ui-client';
export const AUTH_SCOPES = 'admin:read admin:write read:accounts';

export function getMonsteraServerUrl() {
    // NEXT_PUBLIC_MONSTERA_SERVER_URL is a build time variable used for local development.
    // Specifically, it is used accomodate the UI when running outside of Docker.
    if (process.env.NEXT_PUBLIC_MONSTERA_SERVER_URL) {
        return process.env.NEXT_PUBLIC_MONSTERA_SERVER_URL;
    }
    return window.location.origin;
}

export function getDashboardRedirectUri() {
    return `${window.location.origin}/dashboard`;
}
