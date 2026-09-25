# syntax=docker/dockerfile:1.7
#
# Scouter PHP app image (web UI/API + background workers + cron scheduler).
# Published as davaxi/scouter-app — see .github/workflows/docker-publish.yml.
#
# Stage 1 assembles /app (sources + vendor/) with the official composer image,
# so Composer never ships in the runtime image. The final stage adds only two
# layers on top of php:8.3-fpm: one RUN (system packages, PHP extensions and
# config files, bind-mounted from docker/ so they cost no extra layer) and one
# COPY of the app.

# ---------------------------------------------------------------- vendor ----
FROM composer:2 AS vendor
WORKDIR /app
# 1 (default) drops require-dev (pest…) from production images. The local
# compose file passes 0 so `docker exec scouter ./vendor/bin/pest` works.
ARG COMPOSER_NO_DEV=1
# Dependencies first: this layer is reused as long as composer.lock is unchanged.
# The extensions required by the lock are checked for real at runtime by
# vendor/composer/platform_check.php; the composer image doesn't need them.
COPY composer.json composer.lock ./
RUN --mount=type=cache,target=/tmp/cache \
    COMPOSER_NO_DEV=${COMPOSER_NO_DEV} composer install \
      --no-interaction --prefer-dist --no-progress --no-scripts --no-autoloader \
      --ignore-platform-req='ext-*'
COPY . .
RUN COMPOSER_NO_DEV=${COMPOSER_NO_DEV} composer dump-autoload --optimize --no-interaction

# --------------------------------------------------------------- runtime ----
FROM php:8.3-fpm

# curl, dom, mbstring, pdo and pdo_sqlite are compiled into the official image;
# only pdo_pgsql, zip and pcntl are missing. install-php-extensions pulls the
# build deps, compiles, then purges them while keeping the runtime libs.
RUN --mount=type=bind,from=mlocati/php-extension-installer:2,source=/usr/bin/install-php-extensions,target=/usr/local/bin/install-php-extensions \
    --mount=type=bind,source=docker,target=/tmp/docker \
    set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends curl nginx supervisor cron; \
    install-php-extensions pdo_pgsql zip pcntl; \
    apt-get clean; \
    rm -rf /var/lib/apt/lists/* /tmp/pear; \
    \
    install -m 0644 /tmp/docker/nginx.conf /etc/nginx/sites-available/default; \
    install -m 0644 /tmp/docker/supervisord.prod.conf /etc/supervisor/conf.d/supervisord.conf; \
    install -m 0755 /tmp/docker/entrypoint.sh /entrypoint.sh; \
    \
    printf "0 * * * * root . /etc/environment; /usr/local/bin/php /app/scripts/watchdog.php >> /proc/1/fd/1 2>> /proc/1/fd/2\n* * * * * root . /etc/environment; /usr/local/bin/php /app/app/bin/scheduler.php >> /proc/1/fd/1 2>> /proc/1/fd/2\n" > /etc/cron.d/scouter-cron; \
    chmod 0644 /etc/cron.d/scouter-cron; \
    crontab /etc/cron.d/scouter-cron; \
    \
    # Unlimited execution time for PHP-FPM
    printf 'max_execution_time = 0\nmemory_limit = -1\ndefault_socket_timeout = 3600\n' \
      > /usr/local/etc/php/conf.d/timeout.ini; \
    \
    # Tune PHP-FPM pool : default config ships with `pm.max_children = 5` which
    # is way too low when long-running endpoints (SSE chat) coexist with normal
    # requests. With 5 workers, a single in-flight Dr. Brief conversation
    # already eats 20% of the pool ; 3 of them and the whole app freezes for
    # every user. We bump it to 40 dynamic workers — generous headroom for
    # small/medium installs without exploding RAM (~150 MB/worker peak).
    printf '%s\n' \
      '[www]' \
      'pm = dynamic' \
      'pm.max_children = 40' \
      'pm.start_servers = 8' \
      'pm.min_spare_servers = 4' \
      'pm.max_spare_servers = 16' \
      'pm.max_requests = 500' \
      'request_terminate_timeout = 0' \
      > /usr/local/etc/php-fpm.d/zz-scouter.conf; \
    \
    mkdir -p /var/log/supervisor; \
    chown -R www-data:www-data /var/log/nginx

# --chown/--chmod here instead of a later `chown -R` + `chmod -R`, which
# duplicated the whole /app tree into an extra layer. Copied before WORKDIR so
# /app itself is created www-data-owned too.
COPY --from=vendor --chown=www-data:www-data --chmod=755 /app /app
WORKDIR /app

EXPOSE 8080

CMD ["/entrypoint.sh"]
