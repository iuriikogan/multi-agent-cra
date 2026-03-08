/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  output: 'export',
  // Required for static export to work with dynamic routes if any (not strictly needed for basic app but good practice)
  trailingSlash: true,
}

module.exports = nextConfig
