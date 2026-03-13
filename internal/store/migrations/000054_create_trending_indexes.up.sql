CREATE TABLE trending_statuses (
    status_id  TEXT        NOT NULL PRIMARY KEY,
    score      FLOAT8      NOT NULL,
    ranked_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE trending_tag_history (
    hashtag_id TEXT   NOT NULL,
    day        DATE   NOT NULL,
    uses       BIGINT NOT NULL DEFAULT 0,
    accounts   BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (hashtag_id, day)
);
