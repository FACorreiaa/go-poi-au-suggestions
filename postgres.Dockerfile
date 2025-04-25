FROM postgis/postgis:17-3.5 

# Install build dependencies
RUN apt-get update && apt-get install -y \
    build-essential \
    postgresql-server-dev-17 \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Download and extract pgvector
RUN curl -L https://github.com/pgvector/pgvector/archive/refs/tags/v0.8.0.tar.gz | tar xz -C /tmp

# Build and install pgvector
RUN cd /tmp/pgvector-0.8.0 && make && make install

# Clean up temporary files
RUN rm -rf /tmp/pgvector-0.8.0