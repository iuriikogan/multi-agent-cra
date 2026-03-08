import Head from 'next/head'
import Dashboard from '../components/Dashboard'
import { Container, Typography, Box } from '@mui/material'

export default function Home() {
  return (
    <>
      <Head>
        <title>CRA Compliance Dashboard</title>
        <meta name="description" content="Cyber Resilience Act Compliance Dashboard" />
      </Head>
      <main>
        <Container maxWidth="xl">
          <Box sx={{ my: 4 }}>
            <Typography variant="h4" component="h1" gutterBottom>
              CRA Compliance Overview
            </Typography>
            <Dashboard />
          </Box>
        </Container>
      </main>
    </>
  )
}
