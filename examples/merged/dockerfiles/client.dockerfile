# Use the latest Ubuntu image as the base
FROM ubuntu:latest

# Set the maintainer label
LABEL maintainer="ben@grewelltech.com"

# Update the package list and install necessary packages
RUN apt-get update && \
    apt-get install -y supervisor && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Create a directory for supervisor configuration files
RUN mkdir -p /etc/supervisor/conf.d

# Add a default supervisor configuration file
COPY supervisord.conf /etc/supervisor/conf.d/supervisord.conf

# Create a directory for supervisor logs
RUN mkdir -p /var/log/supervisor

# Add the supervisor configuration file for services
COPY supervisord.conf /etc/supervisor/supervisord.conf

# Expose ports if any service needs it (optional)
# EXPOSE 80 443

# Command to start supervisord
CMD ["supervisord", "-c", "/etc/supervisor/supervisord.conf"]
