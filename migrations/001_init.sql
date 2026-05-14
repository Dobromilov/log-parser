CREATE TABLE IF NOT EXISTS logs (
    id UUID PRIMARY KEY,
    file_path TEXT NOT NULL,
    status TEXT NOT NULL,
    error_message TEXT,
    nodes_count INTEGER NOT NULL DEFAULT 0,
    ports_count INTEGER NOT NULL DEFAULT 0,
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    parsed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS nodes (
    id UUID PRIMARY KEY,
    log_id UUID NOT NULL REFERENCES logs(id) ON DELETE CASCADE,
    external_id TEXT NOT NULL,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    num_ports INTEGER NOT NULL,
    node_guid TEXT NOT NULL,
    raw JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (log_id, external_id)
);

CREATE TABLE IF NOT EXISTS ports (
    id UUID PRIMARY KEY,
    log_id UUID NOT NULL REFERENCES logs(id) ON DELETE CASCADE,
    node_id UUID NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    node_guid TEXT NOT NULL,
    port_guid TEXT NOT NULL,
    port_num INTEGER NOT NULL,
    lid TEXT NOT NULL,
    port_phy_state INTEGER NOT NULL,
    port_state_code INTEGER NOT NULL,
    port_state TEXT NOT NULL,
    link_width_active TEXT NOT NULL,
    link_speed_active TEXT NOT NULL,
    raw JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (log_id, node_id, port_num)
);

CREATE TABLE IF NOT EXISTS nodes_info (
    id UUID PRIMARY KEY,
    log_id UUID NOT NULL REFERENCES logs(id) ON DELETE CASCADE,
    node_id UUID NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    node_guid TEXT NOT NULL,
    serial_number TEXT NOT NULL,
    part_number TEXT NOT NULL,
    revision TEXT NOT NULL,
    product_name TEXT NOT NULL,
    raw JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (log_id, node_id)
);

CREATE INDEX IF NOT EXISTS idx_nodes_log_id ON nodes(log_id);
CREATE INDEX IF NOT EXISTS idx_ports_log_id ON ports(log_id);
CREATE INDEX IF NOT EXISTS idx_ports_node_id ON ports(node_id);
CREATE INDEX IF NOT EXISTS idx_nodes_info_log_id ON nodes_info(log_id);
