CREATE TABLE trending_link_history (
    url        TEXT   NOT NULL,
    day        DATE   NOT NULL,
    uses       BIGINT NOT NULL DEFAULT 0,
    accounts   BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (url, day)
);

CREATE TABLE trending_links (
    url        TEXT        NOT NULL PRIMARY KEY,
    score      FLOAT8      NOT NULL,
    ranked_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE trending_link_allowlist (
    url        TEXT        NOT NULL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE trending_link_denylist (
    url        TEXT        NOT NULL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
