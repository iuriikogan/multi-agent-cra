/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  output: 'export',
  experimental: {
    instrumentationHook: true,
  },
}

module.exports = nextConfig
