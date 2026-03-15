import { useState, useEffect, useRef } from 'react';
import { Chart as ChartJS, ArcElement, Tooltip, Legend } from 'chart.js';
import { Doughnut } from 'react-chartjs-2';
import { 
  Grid, Paper, Typography, Box, TextField, Button, 
  Table, TableBody, TableCell, TableContainer, TableHead, TableRow,
  Chip, CircularProgress, Alert, Fade, Stack
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

export default function Dashboard() {
  const [scope, setScope] = useState('projects/crc-demos-489818');
  const [regulation, setRegulation] = useState('CRA');
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
        body: JSON.stringify({ scope, regulation }),
      });
      if (!res.ok) throw new Error('Failed to start scan');
      const data = await res.json();
      setJobId(data.job_id);
    } catch (err: any) {
      setError(err.message);
      setLoading(false);
    }
  };

  const isCompliant = (s: string) => ['true', 'compliant', 'approved', 'conformant'].includes(s.toLowerCase());
  const isNonCompliant = (s: string) => ['false', 'non-compliant', 'failed', 'rejected', 'non-conformant'].includes(s.toLowerCase());

  const findings = scanResult?.findings || [];
  const compliantCount = findings.filter(f => isCompliant(f.status)).length;
  const nonCompliantCount = findings.filter(f => isNonCompliant(f.status)).length;
  const totalCount = findings.length;
  
  const chartData = {
    labels: ['Conformant', 'Non-Conformant'],
    datasets: [{
      data: [compliantCount, nonCompliantCount],
      backgroundColor: ['#34a85399', '#d9302599'],
      borderWidth: 0,
      hoverOffset: 4,
    }],
  };

  return (
    <Box>
      <Grid container spacing={4}>
        <Grid item xs={12}>
          <Paper sx={{ p: 4, display: 'flex', alignItems: 'center', gap: 3 }}>
            <Box sx={{ flexGrow: 1 }}>
              <TextField 
                label="Assessment Scope" 
                placeholder="e.g. projects/my-project"
                variant="outlined" 
                fullWidth 
                value={scope}
                onChange={(e) => setScope(e.target.value)}
                disabled={loading}
                size="small"
              />
            </Box>
            <Box sx={{ minWidth: 120 }}>
              <TextField
                select
                label="Regulation"
                value={regulation}
                onChange={(e) => setRegulation(e.target.value)}
                disabled={loading}
                size="small"
                fullWidth
                SelectProps={{ native: true }}
              >
                <option value="CRA">CRA</option>
                <option value="DORA">DORA</option>
              </TextField>
            </Box>
            <Button
              variant="contained" 
              startIcon={loading ? <CircularProgress size={20} color="inherit" /> : <PlayArrowIcon />}
              onClick={handleScan}
              disabled={loading}
              sx={{ px: 4 }}
            >
              {loading ? 'Running...' : 'Run New Scan'}
            </Button>
          </Paper>
          {error && <Alert severity="error" sx={{ mt: 2 }}>{error}</Alert>}
        </Grid>

        <Grid item xs={12} md={4}>
          <Paper sx={{ height: 500, display: 'flex', flexDirection: 'column', overflow: 'hidden', backgroundColor: '#202124' }}>
            <Box sx={{ backgroundColor: '#202124', p: 1.5, borderBottom: '1px solid #3c4043', display: 'flex', alignItems: 'center', gap: 1 }}>
              <TerminalIcon sx={{ color: '#9aa0a6', fontSize: 18 }} />
              <Typography variant="caption" sx={{ color: '#9aa0a6', fontFamily: 'monospace', fontWeight: 600 }}>
                agent_execution.log
              </Typography>
            </Box>
            <Box sx={{ flexGrow: 1, overflowY: 'auto', p: 2, fontFamily: 'monospace', fontSize: '0.85rem' }}>
              {events.length === 0 && <Typography variant="caption" sx={{ color: '#5f6368' }}>$ Waiting for telemetry stream...</Typography>}
              {events.slice().map((ev, i) => (
                <Box key={i} sx={{ mb: 1 }}>
                  <span style={{ color: '#5f6368' }}>[{new Date(ev.timestamp).toLocaleTimeString()}]</span>{' '}
                  <span style={{ color: '#8ab4f8' }}>{ev.agent_name}</span>{' '}
                  <span style={{ color: ev.status === 'started' ? '#fde293' : '#81c995' }}>{ev.status}</span>{' '}
                  {ev.resource_name && <span style={{ color: '#9aa0a6' }}>({ev.resource_name.split('/').pop()})</span>}
                </Box>
              ))}
              <div ref={logEndRef} />
            </Box>
          </Paper>
        </Grid>

        <Grid item xs={12} md={8}>
          <Grid container spacing={3}>
            <Grid item xs={12} sm={6}>
              <Paper sx={{ p: 4, display: 'flex', alignItems: 'center', justifyContent: 'space-between', height: '100%' }}>
                <Box>
                  <Typography variant="subtitle2" color="text.secondary" gutterBottom>Scan Progress</Typography>
                  <Typography variant="h5" sx={{ mb: 1 }}>
                    {scanResult ? (scanResult.status === 'completed' ? 'Finished' : 'Processing...') : (jobId ? 'In Progress' : 'Ready')}
                  </Typography>
                  {jobId && <Typography variant="caption" color="text.secondary" sx={{ fontFamily: 'monospace' }}>ID: {jobId.slice(0, 8)}...</Typography>}
                </Box>
                {scanResult && scanResult.status === 'completed' && (
                  <Box sx={{ width: 80, height: 80 }}>
                    <Doughnut data={chartData} options={{ maintainAspectRatio: false, plugins: { legend: { display: false } }, cutout: '70' }} />
                  </Box>
                )}
              </Paper>
            </Grid>
            <Grid item xs={12} sm={6}>
              <Paper sx={{ p: 4, height: '100%', display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
                <Typography variant="subtitle2" color="text.secondary" gutterBottom>Quick Metrics</Typography>
                <Stack direction="row" spacing={4} sx={{ mt: 1 }}>
                  <Box>
                    <Typography variant="h4" color="primary.main">{totalCount}</Typography>
                    <Typography variant="caption" color="text.secondary">Total</Typography>
                  </Box>
                  <Box>
                    <Typography variant="h4" color="secondary.main">{compliantCount}</Typography>
                    <Typography variant="caption" color="text.secondary">Pass</Typography>
                  </Box>
                  <Box>
                    <Typography variant="h4" color="error.main">{nonCompliantCount}</Typography>
                    <Typography variant="caption" color="text.secondary">Fail</Typography>
                  </Box>
                </Stack>
              </Paper>
            </Grid>

            <Grid item xs={12}>
              <Fade in={true}>
                <Paper sx={{ overflow: 'hidden' }}>
                  <Box sx={{ p: 2, borderBottom: '1px solid #dadce0', display: 'flex', alignItems: 'center', gap: 1 }}>
                    <AssessmentIcon sx={{ color: 'primary.main', fontSize: 20 }} />
                    <Typography variant="subtitle1">Scan Details</Typography>
                  </Box>
                  <TableContainer sx={{ maxHeight: 330 }}>
                    <Table stickyHeader size="medium">
                      <TableHead>
                        <TableRow>
                          <TableCell>Resource</TableCell>
                          <TableCell align="center">Status</TableCell>
                          <TableCell>Details</TableCell>
                        </TableRow>
                      </TableHead>
                      <TableBody>
                        {findings.length > 0 ? findings.map((f, idx) => (
                          <TableRow key={idx} hover>
                            <TableCell sx={{ fontFamily: 'monospace', fontSize: '0.8rem' }}>
                              {f.resource_name.split('/').pop()}
                            </TableCell>
                            <TableCell align="center">
                              <Chip
                                label={f.status}
                                variant="outlined"
                                size="small"
                                color={isCompliant(f.status) ? 'success' : 'error'}
                                sx={{ fontWeight: 600 }}
                              />
                            </TableCell>
                            <TableCell sx={{ fontSize: '0.85rem' }}>{f.details}</TableCell>
                          </TableRow>
                        )) : (
                          <TableRow>
                            <TableCell colSpan={3} align="center" sx={{ py: 6, color: 'text.secondary' }}>
                              {loading ? 'Analyzing resources...' : 'No telemetry data received.'}
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
  );
}