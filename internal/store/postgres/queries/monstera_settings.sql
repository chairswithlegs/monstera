-- name: GetMonsteraSettings :one
SELECT id, registration_mode, invite_max_uses, invite_expires_in_days,
       server_name, server_description, server_rules, trending_links_scope
FROM monstera_settings WHERE id = 'default';

-- name: UpdateMonsteraSettings :exec
INSERT INTO monstera_settings
  (id, registration_mode, invite_max_uses, invite_expires_in_days,
   server_name, server_description, server_rules, trending_links_scope)
VALUES ('default', @registration_mode, @invite_max_uses, @invite_expires_in_days,
        @server_name, @server_description, @server_rules, @trending_links_scope)
ON CONFLICT (id) DO UPDATE SET
  registration_mode      = @registration_mode,
  invite_max_uses        = @invite_max_uses,
  invite_expires_in_days = @invite_expires_in_days,
  server_name            = @server_name,
  server_description     = @server_description,
  server_rules           = @server_rules,
  trending_links_scope   = @trending_links_scope;
