/**
 * Rationale: Implements the UI/UX or domain logic for the Next.js frontend, adhering to
 * React functional component paradigms and Material UI design specifications.
 * Terminology: CRA Dashboard, SSR (Server-Side Rendering), Component.
 * Measurability: Enhances user interaction by providing responsive, accessible interfaces.
 */
import { useState, useEffect, useRef } from 'react';
import { Chart as ChartJS, ArcElement, Tooltip, Legend } from 'chart.js';
import { Doughnut } from 'react-chartjs-2';
import { 
  Grid, Paper, Typography, Box, TextField, Button, 
  Table, TableBody, TableCell, TableContainer, TableHead, TableRow,
  Chip, CircularProgress, Alert, List, ListItem, ListItemText, Divider
} from '@mui/material';

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
  const [scope, setScope] = useState('projects/test-project');
  const [jobId, setJobId] = useState<string | null>(null);
  const [scanResult, setScanResult] = useState<ScanResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [events, setEvents] = useState<MonitoringEvent[]>([]);
  const logEndRef = useRef<HTMLDivElement>(null);

  // SSE for Real-time Monitoring
  useEffect(() => {
    const eventSource = new EventSource('/api/stream');
    
    eventSource.onmessage = (event) => {
      try {
        const data: MonitoringEvent = JSON.parse(event.data);
        setEvents((prev) => [data, ...prev].slice(0, 100)); // Keep last 100
        
        // If event belongs to our current scan, refresh result
        if (jobId && data.job_id === jobId && data.status === 'completed' && data.agent_name === 'ResourceTagger') {
           fetchScanResult(jobId);
        }
      } catch (err) {
        // Log parsing errors but continue listening to the stream to maintain live updates.
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
        // Update loading state only if the scan has reached a terminal state (completed or failed).
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

  // Calculate stats using case-insensitive comparison for cross-agent compatibility
  const compliantCount = scanResult?.findings.filter(f => {
    const s = f.status.toLowerCase();
    return s === 'true' || s === 'compliant' || s === 'approved';
  }).length || 0;
  const nonCompliantCount = scanResult?.findings.filter(f => {
    const s = f.status.toLowerCase();
    return s === 'false' || s === 'non-compliant' || s === 'failed' || s === 'rejected';
  }).length || 0;
  
  const chartData = {
    labels: ['Compliant', 'Non-Compliant'],
    datasets: [
      {
        data: [compliantCount, nonCompliantCount],
        backgroundColor: ['rgba(75, 192, 192, 0.2)', 'rgba(255, 99, 132, 0.2)'],
        borderColor: ['rgba(75, 192, 192, 1)', 'rgba(255, 99, 132, 1)'],
        borderWidth: 1,
      },
    ],
  };

  return (
    <Grid container spacing={3}>
      {/* Control Panel / Chat Input */}
      <Grid item xs={12}>
        <Paper sx={{ p: 2, display: 'flex', flexDirection: 'column', backgroundColor: '#f5f5f5' }}>
          <Typography component="h2" variant="h6" color="primary" gutterBottom>
            Security Assistant
          </Typography>
          <Box sx={{ display: 'flex', gap: 2, alignItems: 'center' }}>
            <TextField 
              label="Initiate CRA Scan for Scope" 
              placeholder="e.g. projects/my-project"
              variant="outlined" 
              fullWidth 
              value={scope}
              onChange={(e) => setScope(e.target.value)}
              disabled={loading}
              sx={{ backgroundColor: 'white' }}
            />
            <Button 
              variant="contained" 
              onClick={handleScan} 
              disabled={loading}
              sx={{ minWidth: 120, height: 56 }}
            >
              {loading ? <CircularProgress size={24} color="inherit" /> : 'Run Scan'}
            </Button>
          </Box>
          {error && <Alert severity="error" sx={{ mt: 2 }}>{error}</Alert>}
        </Paper>
      </Grid>

      {/* Live Agent Logs */}
      <Grid item xs={12} md={4}>
        <Paper sx={{ p: 2, height: 500, display: 'flex', flexDirection: 'column' }}>
          <Typography variant="h6" gutterBottom color="secondary">
            Live Agent Execution
          </Typography>
          <Box sx={{ flexGrow: 1, overflowY: 'auto', backgroundColor: '#1e1e1e', color: '#00ff00', p: 1, fontFamily: 'monospace', fontSize: '0.8rem' }}>
            {events.length === 0 && <Typography variant="caption" color="gray">Waiting for events...</Typography>}
            {events.map((ev, i) => (
              <div key={i} style={{ marginBottom: 4 }}>
                [{new Date(ev.timestamp).toLocaleTimeString()}] <b>{ev.agent_name}</b>: {ev.status} 
                {ev.resource_name && <span style={{ color: '#aaa' }}> - {ev.resource_name}</span>}
              </div>
            ))}
            <div ref={logEndRef} />
          </Box>
        </Paper>
      </Grid>

      {/* Stats and Findings */}
      <Grid item xs={12} md={8}>
        <Grid container spacing={3}>
          <Grid item xs={12}>
            <Paper sx={{ p: 2, display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
               <Box>
                  <Typography variant="h6">Status: <Chip label={scanResult?.status || (jobId ? 'In Progress' : 'Idle')} color={scanResult?.status === 'completed' ? 'success' : 'primary'} /></Typography>
                  <Typography variant="caption">Job ID: {jobId || 'N/A'}</Typography>
               </Box>
               <Box sx={{ width: 100, height: 100 }}>
                  <Doughnut data={chartData} options={{ maintainAspectRatio: false, plugins: { legend: { display: false } } }} />
               </Box>
            </Paper>
          </Grid>
          <Grid item xs={12}>
            <Paper sx={{ p: 2, minHeight: 345 }}>
              <Typography variant="h6" gutterBottom>Findings Detail</Typography>
              {scanResult && scanResult.findings.length > 0 ? (
                <TableContainer>
                  <Table size="small">
                    <TableHead>
                      <TableRow>
                        <TableCell>Resource</TableCell>
                        <TableCell>Status</TableCell>
                        <TableCell>Details</TableCell>
                      </TableRow>
                    </TableHead>
                    <TableBody>
                      {scanResult.findings.map((f, idx) => (
                        <TableRow key={idx}>
                          <TableCell>{f.resource_name}</TableCell>
                          <TableCell>
                            <Chip
                              label={f.status}
                              color={(f.status.toLowerCase() === 'true' || f.status.toLowerCase() === 'compliant' || f.status.toLowerCase() === 'approved') ? 'success' :
                                (f.status.toLowerCase() === 'false' || f.status.toLowerCase() === 'non-compliant' || f.status.toLowerCase() === 'failed' || f.status.toLowerCase() === 'rejected') ? 'error' : 'default'}
                              size="small"
                            />
                          </TableCell>
                          <TableCell>{f.details}</TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </TableContainer>
              ) : (
                <Typography color="text.secondary">No findings available.</Typography>
              )}
            </Paper>
          </Grid>
        </Grid>
      </Grid>
    </Grid>
  );
}