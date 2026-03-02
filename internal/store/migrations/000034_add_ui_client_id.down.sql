-- Remove tokens and codes that reference the UI client app, then remove the app
DELETE FROM oauth_access_tokens
WHERE application_id IN (SELECT id FROM oauth_applications WHERE client_id = 'ui-client');
DELETE FROM oauth_authorization_codes
WHERE application_id IN (SELECT id FROM oauth_applications WHERE client_id = 'ui-client');
DELETE FROM oauth_applications WHERE client_id = 'ui-client';