
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

    // Attempt to load config.json from the server
    const response = await fetch('/config.json');
    const data = await response.json();
    
    // Merge the data with the default config
    const newConfig = defaultConfig;
    for (const key in data) {
        if (key in newConfig) {
            newConfig[key as keyof Config] = data[key as keyof Config];
        }
    }

    // Ensure the config is valid
    validateConfig(newConfig);

    // Cache and return
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
