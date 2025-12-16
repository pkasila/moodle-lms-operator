# Dockerfile for a custom Moodle image built from source.
# This approach avoids external dependencies like Bitnami and provides full control.

# --- Build Stage ---
# This stage only downloads the Moodle source code to keep the final image clean.
FROM php:8.4-fpm as builder

# Set Moodle version argument
ARG MOODLE_VERSION=MOODLE_501_STABLE
ENV DEBIAN_FRONTEND=noninteractive

# Install git to clone the repository
RUN apt-get update && apt-get install -y --no-install-recommends git && rm -rf /var/lib/apt/lists/*

# Download Moodle source code from the official repository
WORKDIR /usr/src
RUN git clone --depth 1 -b ${MOODLE_VERSION} https://github.com/moodle/moodle.git

# --- Final Stage ---
# Start from a fresh PHP-FPM image for the final "runner"
FROM php:8.4-fpm

ENV DEBIAN_FRONTEND=noninteractive

# Install required system dependencies for Moodle, PHP extensions, nginx, and memcached
RUN apt-get update && apt-get install -y --no-install-recommends \
    unzip \
    libzip-dev \
    libpng-dev \
    libjpeg-dev \
    libfreetype6-dev \
    libicu-dev \
    libpq-dev \
    libxml2-dev \
    libmemcached-dev \
    zlib1g-dev \
    nginx \
    supervisor \
    && rm -rf /var/lib/apt/lists/*

# Install the PHP extensions required by Moodle
# See: https://moodledev.io/docs/apis/server-requirements
RUN docker-php-ext-configure gd --with-freetype --with-jpeg \
    && docker-php-ext-install -j$(nproc) \
    gd \
    intl \
    opcache \
    pdo_pgsql \
    pgsql \
    soap \
    zip \
    exif \
    filter

# Install memcached PHP extension using PECL
RUN pecl install memcached \
    && docker-php-ext-enable memcached

# --- Composer Installation ---
# Install Composer to manage PHP dependencies
COPY --from=composer:lts /usr/bin/composer /usr/bin/composer

# Copy Moodle source from the builder stage
COPY --from=builder /usr/src/moodle /var/www/html

# Set working directory for Composer
WORKDIR /var/www/html

# Install Moodle's Composer dependencies
RUN composer install --no-dev --classmap-authoritative

# --- Custom PHP Configuration ---
# Create a custom ini file to meet Moodle requirements
RUN echo "zend.exception_ignore_args = On" > /usr/local/etc/php/conf.d/moodle.ini && \
    echo "max_input_vars = 5000" >> /usr/local/etc/php/conf.d/moodle.ini

# Copy the custom Moodle configuration
COPY config.php /var/www/html/config.php

# Create Moodle data directory and set permissions
RUN mkdir -p /var/www/moodledata && chown -R www-data:www-data /var/www/moodledata /var/www/html

# --- NGINX Configuration ---
# Remove default NGINX config and install our custom one
RUN rm -f /etc/nginx/sites-enabled/default
COPY nginx.conf /etc/nginx/sites-available/moodle
RUN ln -s /etc/nginx/sites-available/moodle /etc/nginx/sites-enabled/moodle \
    && mkdir -p /var/log/nginx /var/lib/nginx/body /var/lib/nginx/fastcgi /var/cache/nginx \
    && sed -i 's|pid /run/nginx.pid;|pid /tmp/nginx.pid;|' /etc/nginx/nginx.conf \
    && chown -R www-data:www-data /var/log/nginx /var/lib/nginx /var/cache/nginx

# --- Supervisor Configuration ---
COPY supervisord.conf /etc/supervisor/conf.d/supervisord.conf

# Switch to the non-root user
USER www-data

# Expose port 8080 for HTTP
EXPOSE 8080

# Start supervisor which will manage nginx and php-fpm
CMD ["/usr/bin/supervisord", "-c", "/etc/supervisor/conf.d/supervisord.conf"]
