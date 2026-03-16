import Head from 'next/head';
import { ThemeProvider, CssBaseline, Box, Container, Typography } from '@mui/material';
import theme from '../styles/theme';
import Dashboard from '../components/Dashboard';

export default function Home() {
  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <Head>
        <title>Compliance Dashboard</title>
        <meta name="description" content="Audit Agent Compliance Dashboard" />
        <link rel="icon" href="/favicon.ico" />
      </Head>
      <Box component="main" sx={{ flexGrow: 1, bgcolor: 'background.default', minHeight: '100vh' }}>
        <Dashboard />
      </Box>
    </ThemeProvider>
  );
}

