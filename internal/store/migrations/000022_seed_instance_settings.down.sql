DELETE FROM instance_settings
WHERE key IN (
    'instance_name', 'instance_description', 'registration_mode',
    'contact_email', 'max_status_chars', 'media_max_bytes', 'rules_text'
);
