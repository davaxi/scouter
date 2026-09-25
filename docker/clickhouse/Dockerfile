# ClickHouse with Scouter's server config baked in. Published as
# davaxi/scouter-clickhouse, so production hosts need no bind-mounted files.
#
# config.d/*.xml only: rename a *.xml.disable file to *.xml to enable it
# (ClickHouse ignores other extensions anyway).
FROM clickhouse/clickhouse-server:24-alpine
COPY config.d/*.xml /etc/clickhouse-server/config.d/
