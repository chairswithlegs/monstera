INSERT INTO oauth_applications (id, client_id, client_secret, name, redirect_uris, scopes, website)
VALUES (
  gen_random_uuid(),
  'ui-client',
  '',
  'Monstera UI Client',
  '',
  'admin:read admin:write',
  ''
);
