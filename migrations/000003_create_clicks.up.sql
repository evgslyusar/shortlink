CREATE TABLE clicks (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    link_id    UUID        NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    clicked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    country    CHAR(2),
    referer    TEXT,
    user_agent TEXT
);

CREATE INDEX idx_clicks_link_id    ON clicks(link_id);
CREATE INDEX idx_clicks_clicked_at ON clicks(link_id, clicked_at);
