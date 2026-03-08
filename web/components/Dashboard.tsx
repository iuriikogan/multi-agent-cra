import { useState, useEffect } from 'react';
import { Chart as ChartJS, ArcElement, Tooltip, Legend } from 'chart.js';
import { Doughnut } from 'react-chartjs-2';
import { 
  Grid, Paper, Typography, Box, TextField, Button, 
  Table, TableBody, TableCell, TableContainer, TableHead, TableRow,
  Chip, CircularProgress, Alert
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

export default function Dashboard() {
  const [scope, setScope] = useState('projects/test-project');
  const [jobId, setJobId] = useState<string | null>(null);
  const [scanResult, setScanResult] = useState<ScanResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Poll for updates if we have a job ID and status is not final
  useEffect(() => {
    if (!jobId) return;

    const poll = async () => {
      try {
        const res = await fetch(`/api/scan?id=${jobId}`);
        if (!res.ok) throw new Error('Failed to fetch scan results');
        const data: ScanResult = await res.json();
        setScanResult(data);
        
        if (data.status === 'completed' || data.status === 'failed') {
          setLoading(false);
        }
      } catch (err) {
        console.error(err);
        // Don't stop polling immediately on one error, but maybe warn?
      }
    };

    // Initial fetch
    poll();

    // Poll every 3 seconds
    const interval = setInterval(() => {
      if (scanResult?.status !== 'completed' && scanResult?.status !== 'failed') {
        poll();
      } else {
        clearInterval(interval);
      }
    }, 3000);

    return () => clearInterval(interval);
  }, [jobId, scanResult?.status]);

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

      if (!res.ok) {
        throw new Error('Failed to start scan');
      }

      const data = await res.json();
      setJobId(data.job_id);
    } catch (err: any) {
      setError(err.message);
      setLoading(false);
    }
  };

  // Calculate stats from real findings
  const compliantCount = scanResult?.findings.filter(f => f.status === 'true' || f.status === 'Compliant').length || 0;
  const nonCompliantCount = scanResult?.findings.filter(f => f.status === 'false' || f.status === 'Non-Compliant').length || 0;
  
  // If no findings yet, show empty or placeholder if needed
  const chartData = {
    labels: ['Compliant', 'Non-Compliant'],
    datasets: [
      {
        data: [compliantCount, nonCompliantCount],
        backgroundColor: [
          'rgba(75, 192, 192, 0.2)',
          'rgba(255, 99, 132, 0.2)',
        ],
        borderColor: [
          'rgba(75, 192, 192, 1)',
          'rgba(255, 99, 132, 1)',
        ],
        borderWidth: 1,
      },
    ],
  };

  return (
    <Grid container spacing={3}>
      {/* Control Panel */}
      <Grid item xs={12}>
        <Paper sx={{ p: 2, display: 'flex', flexDirection: 'column' }}>
          <Typography component="h2" variant="h6" color="primary" gutterBottom>
            Start New Scan
          </Typography>
          <Box sx={{ display: 'flex', gap: 2, alignItems: 'center' }}>
            <TextField 
              label="Scope (e.g. projects/my-project)" 
              variant="outlined" 
              fullWidth 
              value={scope}
              onChange={(e) => setScope(e.target.value)}
              disabled={loading}
            />
            <Button 
              variant="contained" 
              onClick={handleScan} 
              disabled={loading}
              sx={{ minWidth: 120, height: 56 }}
            >
              {loading ? <CircularProgress size={24} /> : 'Scan'}
            </Button>
          </Box>
          {error && <Alert severity="error" sx={{ mt: 2 }}>{error}</Alert>}
          {jobId && (
            <Alert severity="info" sx={{ mt: 2 }}>
              Job ID: {jobId} | Status: {scanResult?.status || 'Queued'}
            </Alert>
          )}
        </Paper>
      </Grid>

      {/* Stats Chart */}
      <Grid item xs={12} md={4}>
        <Paper sx={{ p: 2, display: 'flex', flexDirection: 'column', height: 300 }}>
          <Typography component="h2" variant="h6" color="primary" gutterBottom>
            Compliance Status
          </Typography>
          <div style={{ position: 'relative', height: '100%', width: '100%', display: 'flex', justifyContent: 'center' }}>
            {scanResult && scanResult.findings.length > 0 ? (
              <Doughnut data={chartData} options={{ maintainAspectRatio: false }} />
            ) : (
              <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%' }}>
                <Typography color="text.secondary">No data available</Typography>
              </Box>
            )}
          </div>
        </Paper>
      </Grid>

      {/* Findings List */}
      <Grid item xs={12} md={8}>
        <Paper sx={{ p: 2, display: 'flex', flexDirection: 'column', minHeight: 300 }}>
          <Typography component="h2" variant="h6" color="primary" gutterBottom>
            Findings
          </Typography>
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
                  {scanResult.findings.map((finding, idx) => (
                    <TableRow key={idx}>
                      <TableCell>{finding.resource_name}</TableCell>
                      <TableCell>
                        <Chip 
                          label={finding.status} 
                          color={finding.status === 'true' || finding.status === 'Compliant' ? 'success' : 'error'} 
                          size="small" 
                        />
                      </TableCell>
                      <TableCell>{finding.details}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </TableContainer>
          ) : (
            <Typography variant="body2" sx={{ mt: 2 }}>
              No findings to display. Start a scan to see results.
            </Typography>
          )}
        </Paper>
      </Grid>
    </Grid>
  );
}