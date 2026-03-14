import Head from 'next/head'
import { useState, SyntheticEvent } from 'react'
import { Container, Typography, Box, Tabs, Tab } from '@mui/material'
import Dashboard from '../components/Dashboard'
import CRADashboard from '../components/CRADashboard'

// TabPanelProps defines the structure for custom tab content panels.
interface TabPanelProps {
  children?: React.ReactNode;
  index: number;
  value: number;
}

// CustomTabPanel renders children only when the associated tab is selected.
function CustomTabPanel(props: TabPanelProps) {
  const { children, value, index, ...other } = props;

  return (
    <div
      role="tabpanel"
      hidden={value !== index}
      id={`simple-tabpanel-${index}`}
      aria-labelledby={`simple-tab-${index}`}
      {...other}
    >
      {value === index && (
        <Box sx={{ p: 3 }}>
          {children}
        </Box>
      )}
    </div>
  );
}

// a11yProps generates accessibility attributes for tab elements.
function a11yProps(index: number) {
  return {
    id: `simple-tab-${index}`,
    'aria-controls': `simple-tabpanel-${index}`,
  };
}

// Home serves as the main entry point for the Next.js compliance dashboard.
export default function Home() {
  const [value, setValue] = useState(0);

  const handleChange = (event: SyntheticEvent, newValue: number) => {
    setValue(newValue);
  };

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
              CRA Compliance System
            </Typography>
            
            <Box sx={{ borderBottom: 1, borderColor: 'divider' }}>
              <Tabs value={value} onChange={handleChange} aria-label="cra dashboard tabs">
                <Tab label="CRA Dashboard" {...a11yProps(0)} />
                <Tab label="Live Agent Logs" {...a11yProps(1)} />
              </Tabs>
            </Box>
            
            <CustomTabPanel value={value} index={0}>
              <CRADashboard />
            </CustomTabPanel>
            
            <CustomTabPanel value={value} index={1}>
              <Dashboard />
            </CustomTabPanel>

          </Box>
        </Container>
      </main>
    </>
  )
}
