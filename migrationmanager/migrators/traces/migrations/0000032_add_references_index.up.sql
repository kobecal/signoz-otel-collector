ALTER TABLE signoz_traces.signoz_index_v2 ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD COLUMN IF NOT EXISTS references String CODEC(ZSTD(1));