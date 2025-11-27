FROM nginx:alpine

# Remove default nginx page
RUN rm -rf /usr/share/nginx/html/*

# Copy all files from public to nginx root
COPY ./public/ /usr/share/nginx/html/

# Copy nginx configuration
COPY nginx.conf /etc/nginx/nginx.conf

# Create necessary directories
RUN mkdir -p /usr/share/nginx/html/static /usr/share/nginx/html/src

# Copy static files (if any)
COPY ./static/ /usr/share/nginx/html/static/
COPY ./src/ /usr/share/nginx/html/src/

# Set proper permissions
RUN chown -R nginx:nginx /usr/share/nginx/html

EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]