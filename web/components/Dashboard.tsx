import { useState, useEffect, useRef } from 'react';
import { Chart as ChartJS, ArcElement, Tooltip, Legend } from 'chart.js';
import { Doughnut } from 'react-chartjs-2';
import { 
  Grid, Paper, Typography, Box, TextField, Button, 
  Table, TableBody, TableCell, TableContainer, TableHead, TableRow,
  Chip, CircularProgress, Alert, ThemeProvider, createTheme, Fade
} from '@mui/material';
import SecurityIcon from '@mui/icons-material/Security';
import PlayArrowIcon from '@mui/icons-material/PlayArrow';
import CheckCircleIcon from '@mui/icons-material/CheckCircle';
import ErrorIcon from '@mui/icons-material/Error';
import TerminalIcon from '@mui/icons-material/Terminal';
import AssessmentIcon from '@mui/icons-material/Assessment';

ChartJS.register(ArcElement, Tooltip, Legend);

interface Finding {
  resource_name: string;
  status: string;
  details: string;
}

interface ScanResult {
  job_id: string;
  scope: string;
  status: string;
  findings: Finding[];
  created_at: string;
}

interface MonitoringEvent {
  job_id: string;
  resource_name: string;
  agent_name: string;
  status: string;
  details: string;
  timestamp: string;
}

const theme = createTheme({
  palette: {
    primary: { main: '#6366f1' }, // Indigo
    secondary: { main: '#ec4899' }, // Pink
    background: { default: '#f8fafc', paper: '#ffffff' },
    text: { primary: '#1e293b', secondary: '#64748b' },
  },
  typography: {
    fontFamily: '"Inter", "Roboto", "Helvetica", "Arial", sans-serif',
    h5: { fontWeight: 700 },
    h6: { fontWeight: 600 },
  },
  shape: { borderRadius: 16 },
  components: {
    MuiPaper: {
      styleOverrides: {
        root: {
          boxShadow: '0 4px 6px -1px rgb(0 0 0 / 0.1), 0 2px 4px -2px rgb(0 0 0 / 0.1)',
          border: '1px solid #e2e8f0',
        },
      },
    },
    MuiButton: {
      styleOverrides: {
        root: {
          textTransform: 'none',
          fontWeight: 600,
          borderRadius: 8,
        },
      },
    },
    MuiTableCell: {
      styleOverrides: {
        head: { fontWeight: 600, backgroundColor: '#f8fafc', color: '#475569' },
      },
    },
  },
});

export default function Dashboard() {
  const [scope, setScope] = useState('projects/crc-demos-489818');
  const [jobId, setJobId] = useState<string | null>(null);
  const [scanResult, setScanResult] = useState<ScanResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [events, setEvents] = useState<MonitoringEvent[]>([]);
  const logEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const fetchAllFindings = async () => {
      try {
        const res = await fetch('/api/findings');
        if (res.ok) {
          const findingsData = await res.json();
          setScanResult({
            job_id: 'historical',
            scope: 'all',
            status: 'completed',
            findings: findingsData || [],
            created_at: new Date().toISOString()
          });
        }
      } catch (err) {
        console.error("Error fetching historical findings", err);
      }
    };
    fetchAllFindings();
  }, []);

  // Auto-scroll the terminal
  useEffect(() => {
    if (logEndRef.current) {
      logEndRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [events]);

  useEffect(() => {
    const eventSource = new EventSource('/api/stream');
    eventSource.onmessage = (event) => {
      try {
        const data: MonitoringEvent = JSON.parse(event.data);
        setEvents((prev) => [data, ...prev].slice(0, 100)); 

        if (jobId && data.job_id === jobId && data.status === 'completed' && data.agent_name === 'Reporter') {
           setTimeout(() => fetchScanResult(jobId), 500);
        }
      } catch (err) {
        console.error("Failed to parse SSE data", err);
      }
    };
    eventSource.onerror = (err) => {
      console.error("SSE Error", err);
      eventSource.close();
    };
    return () => eventSource.close();
  }, [jobId]);

  const fetchScanResult = async (id: string) => {
    try {
      const res = await fetch(`/api/scan?id=${id}`);
      if (res.ok) {
        const data: ScanResult = await res.json();
        setScanResult(data);
        if (data.status === 'completed' || data.status === 'failed') {
          setLoading(false);
        }
      }
    } catch (err) {
      console.error("Error fetching scan result", err);
    }
  };

  const handleScan = async () => {
    setLoading(true);
    setError(null);
    setScanResult(null);
    setJobId(null);
    setEvents([]);

    try {
      const res = await fetch('/api/scan', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ scope }),
      });
      if (!res.ok) throw new Error('Failed to start scan');
      const data = await res.json();
      setJobId(data.job_id);
    } catch (err: any) {
      setError(err.message);
      setLoading(false);
    }
  };

  const isCompliant = (s: string) => ['true', 'compliant', 'approved'].includes(s.toLowerCase());
  const isNonCompliant = (s: string) => ['false', 'non-compliant', 'failed', 'rejected'].includes(s.toLowerCase());

  const findings = scanResult?.findings || [];
  const compliantCount = findings.filter(f => isCompliant(f.status)).length;
  const nonCompliantCount = findings.filter(f => isNonCompliant(f.status)).length;
  const totalCount = findings.length;
  
  const chartData = {
    labels: ['Compliant', 'Non-Compliant'],
    datasets: [{
      data: [compliantCount, nonCompliantCount],
      backgroundColor: ['#10b981', '#f43f5e'],
      borderWidth: 0,
      hoverOffset: 4,
    }],
  };

  return (
    <ThemeProvider theme={theme}>
      <Box sx={{ p: 4, minHeight: '100vh', backgroundColor: '#f1f5f9' }}>
        <Box sx={{ mb: 4, display: 'flex', alignItems: 'center', gap: 2 }}>
          <SecurityIcon sx={{ fontSize: 40, color: 'primary.main' }} />
          <Box>
            <Typography variant="h5" color="text.primary">AI Compliance Scanner</Typography>
            <Typography variant="body2" color="text.secondary">Automated Cyber Resilience Act (CRA) Analysis</Typography>
          </Box>
        </Box>

        <Grid container spacing={4}>
          <Grid item xs={12}>
            <Paper sx={{ p: 3, display: 'flex', alignItems: 'center', gap: 3, background: 'linear-gradient(to right, #ffffff, #f8fafc)' }}>
              <TextField 
                label="Target Scope" 
                placeholder="e.g. projects/my-project"
                variant="outlined" 
                fullWidth 
                value={scope}
                onChange={(e) => setScope(e.target.value)}
                disabled={loading}
                sx={{ backgroundColor: 'white', borderRadius: 1 }}
              />
              <Button 
                variant="contained" 
                size="large"
                startIcon={loading ? <CircularProgress size={20} color="inherit"/> : <PlayArrowIcon />}
                onClick={handleScan} 
                disabled={loading}
                sx={{ minWidth: 160, height: 56, boxShadow: 2 }}
              >
                {loading ? 'Scanning...' : 'Run Audit'}
              </Button>
            </Paper>
            {error && <Alert severity="error" sx={{ mt: 2, borderRadius: 2 }}>{error}</Alert>}
          </Grid>

          <Grid item xs={12} md={4}>
            <Paper sx={{ height: 500, display: 'flex', flexDirection: 'column', overflow: 'hidden', border: '1px solid #334155' }}>
              <Box sx={{ backgroundColor: '#1e293b', p: 1.5, display: 'flex', alignItems: 'center', gap: 1 }}>
                <Box sx={{ width: 12, height: 12, borderRadius: '50%', backgroundColor: '#ef4444' }} />
                <Box sx={{ width: 12, height: 12, borderRadius: '50%', backgroundColor: '#f59e0b' }} />
                <Box sx={{ width: 12, height: 12, borderRadius: '50%', backgroundColor: '#10b981' }} />
                <Typography variant="caption" sx={{ color: '#94a3b8', ml: 2, fontFamily: 'monospace', display: 'flex', alignItems: 'center', gap: 1 }}>
                  <TerminalIcon fontSize="small"/> agent_execution.log
                </Typography>
              </Box>
              <Box sx={{ flexGrow: 1, overflowY: 'auto', backgroundColor: '#0f172a', color: '#10b981', p: 2, fontFamily: '"Fira Code", monospace', fontSize: '0.85rem' }}>
                {events.length === 0 && <Typography variant="caption" sx={{ color: '#475569' }}>$ Waiting for telemetry stream...</Typography>}
                {events.slice().reverse().map((ev, i) => (
                  <Box key={i} sx={{ mb: 1, opacity: 0.9 }}>
                    <span style={{ color: '#64748b' }}>[{new Date(ev.timestamp).toLocaleTimeString()}]</span>{' '}
                    <span style={{ color: '#38bdf8', fontWeight: 'bold' }}>{ev.agent_name}</span>{' '}
                    <span style={{ color: ev.status === 'started' ? '#fcd34d' : '#10b981' }}>{ev.status}</span>{' '}
                    {ev.resource_name && <span style={{ color: '#94a3b8' }}>({ev.resource_name.split('/').pop()})</span>}
                  </Box>
                ))}
                <div ref={logEndRef} />
              </Box>
            </Paper>
          </Grid>

          <Grid item xs={12} md={8}>
            <Grid container spacing={3}>
              <Grid item xs={12} sm={6}>
                <Paper sx={{ p: 3, display: 'flex', alignItems: 'center', justifyContent: 'space-between', height: '100%' }}>
                  <Box>
                    <Typography variant="subtitle2" color="text.secondary" gutterBottom>Overall Posture</Typography>
                    <Typography variant="h4" sx={{ fontWeight: 800, color: 'text.primary', mb: 1 }}>
                      {scanResult ? (scanResult.status === 'completed' ? 'Audit Finished' : 'Processing...') : (jobId ? 'In Progress' : 'Ready')}
                    </Typography>
                    {jobId && <Typography variant="caption" color="text.secondary" sx={{ fontFamily: 'monospace' }}>Job: {jobId}</Typography>}
                  </Box>
                  {scanResult && scanResult.status === 'completed' && (
                    <Box sx={{ width: 100, height: 100 }}>
                      <Doughnut data={chartData} options={{ maintainAspectRatio: false, plugins: { legend: { display: false } }, cutout: '75%' }} />
                    </Box>
                  )}
                </Paper>
              </Grid>
              <Grid item xs={12} sm={6}>
                <Paper sx={{ p: 3, height: '100%', display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
                  <Typography variant="subtitle2" color="text.secondary" gutterBottom>Metrics</Typography>
                  <Box sx={{ display: 'flex', justifyContent: 'space-around', mt: 2 }}>
                    <Box sx={{ textAlign: 'center' }}>
                      <Typography variant="h3" color="primary.main" fontWeight="bold">{totalCount}</Typography>
                      <Typography variant="caption" color="text.secondary">Total</Typography>
                    </Box>
                    <Box sx={{ textAlign: 'center' }}>
                      <Typography variant="h3" sx={{ color: '#10b981', fontWeight: 'bold' }}>{compliantCount}</Typography>
                      <Typography variant="caption" color="text.secondary">Secure</Typography>
                    </Box>
                    <Box sx={{ textAlign: 'center' }}>
                      <Typography variant="h3" sx={{ color: '#f43f5e', fontWeight: 'bold' }}>{nonCompliantCount}</Typography>
                      <Typography variant="caption" color="text.secondary">Violations</Typography>
                    </Box>
                  </Box>
                </Paper>
              </Grid>

              <Grid item xs={12}>
                <Fade in={true}>
                  <Paper sx={{ overflow: 'hidden' }}>
                    <Box sx={{ p: 2, borderBottom: '1px solid #e2e8f0', display: 'flex', alignItems: 'center', gap: 1 }}>
                      <AssessmentIcon color="primary" />
                      <Typography variant="h6">Compliance Report</Typography>
                    </Box>
                    <TableContainer sx={{ maxHeight: 400 }}>
                      <Table stickyHeader size="medium">
                        <TableHead>
                          <TableRow>
                            <TableCell>Resource Target</TableCell>
                            <TableCell align="center">Compliance</TableCell>
                            <TableCell>Auditor Notes</TableCell>
                          </TableRow>
                        </TableHead>
                        <TableBody>
                          {findings.length > 0 ? findings.map((f, idx) => (
                            <TableRow key={idx} hover sx={{ '&:last-child td': { border: 0 } }}>
                              <TableCell sx={{ maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontFamily: 'monospace', fontSize: '0.8rem' }}>
                                {f.resource_name.split('/').pop()}
                              </TableCell>
                              <TableCell align="center">
                                <Chip
                                  icon={isCompliant(f.status) ? <CheckCircleIcon /> : <ErrorIcon />}
                                  label={f.status}
                                  size="small"
                                  sx={{ 
                                    fontWeight: 'bold', 
                                    color: isCompliant(f.status) ? '#065f46' : '#991b1b',
                                    backgroundColor: isCompliant(f.status) ? '#d1fae5' : '#fee2e2' 
                                  }}
                                />
                              </TableCell>
                              <TableCell sx={{ fontSize: '0.85rem', color: 'text.secondary' }}>{f.details}</TableCell>
                            </TableRow>
                          )) : (
                            <TableRow>
                              <TableCell colSpan={3} align="center" sx={{ py: 6, color: 'text.secondary' }}>
                                {loading ? 'Scanning resources...' : 'No security findings generated yet.'}
                              </TableCell>
                            </TableRow>
                          )}
                        </TableBody>
                      </Table>
                    </TableContainer>
                  </Paper>
                </Fade>
              </Grid>
            </Grid>
          </Grid>
        </Grid>
      </Box>
    </ThemeProvider>
  );
}