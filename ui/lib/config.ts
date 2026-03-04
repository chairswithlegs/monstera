
interface Config {
    server_url: string;
    auth_client_id: string;
    auth_scopes: string;
}

let config: Config | null = null;

export async function getConfig(): Promise<Config> {
    // Return the cached config if it exists
    if (config) {
        return config;
    }

    // In development, config.json is not served; use defaults from env.
    // In production, config is loaded from /config.json (e.g. nginx).
    const response = await fetch('/config.json', { cache: 'no-store' });
    const newConfig: Config = { ...defaultConfig };
    if (response.ok) {
        const data = (await response.json()) as Partial<Config>;
        for (const key of Object.keys(data) as (keyof Config)[]) {
            if (key in newConfig && data[key] !== undefined) {
                newConfig[key] = data[key] as Config[keyof Config];
            }
        }
    }

    validateConfig(newConfig);
    config = newConfig;
    return config;
}

const defaultConfig: Config = {
    auth_client_id: 'ui-client',
    auth_scopes: 'admin:read admin:write read:accounts',

    // Environment variables will only be available when running the app locally
    server_url: process.env.NEXT_PUBLIC_SERVER_URL ?? '',
};

function validateConfig(config: Config): Config {
    const errors: string[] = [];
    if (!config.server_url) {
        errors.push('Server URL is required');
    }
    if (!config.auth_client_id) {
        errors.push('Auth client ID is required');
    }
    if (!config.auth_scopes) {
        errors.push('Auth scopes are required');
    }
    if (errors.length > 0) {
        throw new Error(errors.join(', '));
    }
    return config;
}
