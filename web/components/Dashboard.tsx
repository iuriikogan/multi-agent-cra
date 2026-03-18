import React, { useState, useEffect, useMemo, useRef, useCallback } from 'react';
import {
  Box,
  Typography,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Chip,
  Grid,
  Select,
  MenuItem,
  FormControl,
  InputLabel,
  Button,
  CircularProgress,
  SelectChangeEvent,
  Stack,
  Alert,
  Fade,
  TextField,
  Tooltip,
  IconButton,
  AppBar,
  Toolbar,
  Container,
  Tabs,
  Tab,
  useTheme,
  useMediaQuery,
  Card,
  CardContent,
  Divider,
} from '@mui/material';
import {
  Share,
  Download,
  PlayArrow,
  CheckCircle,
  Error as ErrorIcon,
  Terminal,
  Assessment,
  Security,
  Insights,
} from '@mui/icons-material';
import { Chart as ChartJS, ArcElement, Tooltip as ChartTooltip, Legend } from 'chart.js';
import { Doughnut } from 'react-chartjs-2';

ChartJS.register(ArcElement, ChartTooltip, Legend);

// --- Type Definitions ---
interface Finding {
  resource_name: string;
  status: string;
  details: string;
  regulation: string;
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

// --- Helper Functions ---
const extractHierarchy = (resourceName: string) => {
  const parts = resourceName.split('/');
  let org = 'Unknown', folder = 'Unknown', proj = 'Unknown';
  for (let i = 0; i < parts.length - 1; i++) {
    if (parts[i] === 'organizations') org = parts[i + 1];
    else if (parts[i] === 'folders') folder = parts[i + 1];
    else if (parts[i] === 'projects') proj = parts[i + 1];
  }
  return { org, folder, proj };
};

const isCompliant = (status: string) => status.toLowerCase() === 'compliant' || status.toLowerCase() === 'pass';
const isNonCompliant = (status: string) => status.toLowerCase() === 'non_compliant' || status.toLowerCase() === 'non-compliant' || status.toLowerCase() === 'fail';

// --- Main Dashboard Component ---
export default function Dashboard() {
  const [currentTab, setCurrentTab] = useState(0);

  const handleTabChange = (event: React.SyntheticEvent, newValue: number) => {
    setCurrentTab(newValue);
  };

  return (
    <Box sx={{ flexGrow: 1 }}>
      <AppBar position="static" elevation={0} sx={{ backgroundColor: 'background.paper' }}>
        <Toolbar>
          <Security sx={{ mr: 2, color: 'primary.main' }} />
          <Typography variant="h6" component="div" sx={{ flexGrow: 1 }}>
            Compliance Dashboard
          </Typography>
        </Toolbar>
        <Tabs value={currentTab} onChange={handleTabChange} indicatorColor="primary" textColor="primary" variant="fullWidth">
          <Tab label="Compliance Overview" icon={<Assessment />} iconPosition="start" />
          <Tab label="Live Scan & Logs" icon={<Terminal />} iconPosition="start" />
        </Tabs>
      </AppBar>
      <Container maxWidth="xl" sx={{ py: 4 }}>
        <Box sx={{ display: currentTab === 0 ? 'block' : 'none' }}>
          <ComplianceOverview />
        </Box>
        <Box sx={{ display: currentTab === 1 ? 'block' : 'none' }}>
          <LiveScan />
        </Box>
      </Container>
    </Box>
  );
}

// --- Compliance Overview Tab ---
function ComplianceOverview() {
  const [findings, setFindings] = useState<Finding[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filters, setFilters] = useState({
    org: 'All',
    folder: 'All',
    project: 'All',
    regulation: 'All',
  });

  const fetchFindings = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const res = await fetch('/api/findings');
      if (!res.ok) {
        throw new Error(`Failed to fetch findings. Status: ${res.status}`);
      }
      const data = await res.json();
      
      // Data Validation: Ensure we always have an array
      if (Array.isArray(data)) {
        setFindings(data);
      } else if (data && Array.isArray(data.findings)) {
        setFindings(data.findings);
      } else {
        console.warn('Unexpected data format for findings:', data);
        setFindings([]);
      }
    } catch (err: any) {
      console.error("Failed to fetch findings", err);
      setError(err.message || 'An unexpected error occurred while loading findings.');
      setFindings([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchFindings();
  }, [fetchFindings]);

  const filteredFindings = useMemo(() => {
    if (!Array.isArray(findings)) return [];
    
    return findings.filter(f => {
      // Safe guard against malformed data
      if (!f || !f.resource_name) return false;
      
      const { org, folder, proj } = extractHierarchy(f.resource_name);
      if (filters.org !== 'All' && org !== filters.org) return false;
      if (filters.folder !== 'All' && folder !== filters.folder) return false;
      if (filters.project !== 'All' && proj !== filters.project) return false;
      if (filters.regulation !== 'All' && f.regulation !== filters.regulation) return false;
      return true;
    });
  }, [findings, filters]);

  if (loading) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', p: 10 }}>
        <CircularProgress />
      </Box>
    );
  }

  if (error) {
    return (
      <Alert severity="error" action={
        <Button color="inherit" size="small" onClick={fetchFindings}>
          Retry
        </Button>
      }>
        {error}
      </Alert>
    );
  }

  return (
    <Grid container spacing={3}>
      <Grid item xs={12}>
        <FilterControls findings={findings} filters={filters} setFilters={setFilters} />
      </Grid>
      <Grid item xs={12} md={4}>
        <ComplianceChart findings={filteredFindings} />
      </Grid>
      <Grid item xs={12} md={8}>
        <ActionToolbar findings={filteredFindings} />
      </Grid>
      <Grid item xs={12}>
        <FindingsTable findings={filteredFindings} />
      </Grid>
    </Grid>
  );
}

// --- Live Scan Tab ---
function LiveScan() {
  const [scope, setScope] = useState('projects/your-gcp-project-id');
  const [regulation, setRegulation] = useState('CRA');
  const [jobId, setJobId] = useState<string | null>(null);
  const [scanResult, setScanResult] = useState<ScanResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [events, setEvents] = useState<MonitoringEvent[]>([]);
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
    if (typeof window !== 'undefined') {
      const savedScope = localStorage.getItem('audit_scan_scope');
      if (savedScope) setScope(savedScope);
      const savedRegulation = localStorage.getItem('audit_scan_regulation');
      if (savedRegulation) setRegulation(savedRegulation);
      const savedJobId = localStorage.getItem('audit_scan_jobId');
      if (savedJobId) setJobId(savedJobId);
      const savedEvents = localStorage.getItem('audit_scan_events');
      if (savedEvents) setEvents(JSON.parse(savedEvents));
    }
  }, []);

  useEffect(() => {
    if (mounted) {
      localStorage.setItem('audit_scan_scope', scope);
    }
  }, [scope, mounted]);

  useEffect(() => {
    if (mounted) {
      localStorage.setItem('audit_scan_regulation', regulation);
    }
  }, [regulation, mounted]);

  useEffect(() => {
    if (mounted) {
      if (jobId) localStorage.setItem('audit_scan_jobId', jobId);
      else localStorage.removeItem('audit_scan_jobId');
    }
  }, [jobId, mounted]);

  useEffect(() => {
    if (mounted) {
      localStorage.setItem('audit_scan_events', JSON.stringify(events));
    }
  }, [events, mounted]);

  useEffect(() => {
    if (mounted && jobId && !scanResult && !loading) {
      fetchScanResult(jobId);
    }
  }, [mounted, jobId]); // Run once after mount

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
  
  const fetchScanResult = useCallback(async (id: string) => {
    try {
      const res = await fetch(`/api/scan?id=${id}`);
      if (!res.ok) {
        throw new Error(`Failed to fetch scan results: ${res.statusText}`);
      }
      
      const data: ScanResult = await res.json();
      
      // Data validation: Force findings to be an array if it isn't
      if (data && !Array.isArray(data.findings)) {
         console.warn("API returned invalid findings structure. Formatting as array:", data.findings);
         data.findings = []; 
      }
      
      setScanResult(data);
      if (['completed', 'failed'].includes(data.status)) {
        setLoading(false);
      }
    } catch (err: any) {
      console.error("Error fetching scan result", err);
      setError(err.message || 'An error occurred fetching the final scan results.');
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    let reconnectTimeout: NodeJS.Timeout;
    
    const connectSSE = () => {
      const eventSource = new EventSource('/api/stream');
      
      eventSource.onmessage = (event) => {
        try {
          // Data Binding & Real-time Update validation
          const data: MonitoringEvent = JSON.parse(event.data);
          
          if (!data || typeof data !== 'object') return;
          
          setEvents((prev) => [data, ...prev].slice(0, 100)); 

          // Wait until the Reporter claims the job is fully completed before attempting to pull it
          if (jobId && data.job_id === jobId && data.status === 'completed' && data.agent_name === 'Reporter') {
             setTimeout(() => fetchScanResult(jobId), 1000);
          }
          
          // Fallback mechanism: If overall status is failed
          if (jobId && data.job_id === jobId && data.status === 'failed') {
             setTimeout(() => fetchScanResult(jobId), 1000);
          }
        } catch (err) {
          console.error("Failed to parse SSE data", err);
        }
      };
      
      eventSource.onerror = () => {
        eventSource.close();
        // SSE error handling/reconnection logic
        reconnectTimeout = setTimeout(connectSSE, 3000);
      };
      
      return eventSource;
    };
    
    const es = connectSSE();
    return () => {
      es.close();
      clearTimeout(reconnectTimeout);
    };
  }, [jobId, fetchScanResult]);

  if (!mounted) {
    return null;
  }

  return (
    <Grid container spacing={3}>
      <Grid item xs={12}>
        <ScanControls
          scope={scope}
          setScope={setScope}
          regulation={regulation}
          setRegulation={setRegulation}
          loading={loading}
          handleScan={handleScan}
        />
        {error && <Alert severity="error" sx={{ mt: 2 }} onClose={() => setError(null)}>{error}</Alert>}
      </Grid>
      <Grid item xs={12} md={5} lg={4}>
        <AgentLog events={events} />
      </Grid>
      <Grid item xs={12} md={7} lg={8}>
        <ScanResults scanResult={scanResult} loading={loading} jobId={jobId}/>
      </Grid>
    </Grid>
  );
}


// --- Reusable Components ---
function FilterControls({ findings, filters, setFilters }: any) {
  const { orgs, folders, projects } = useMemo(() => {
    const sets = {
      orgs: new Set<string>(),
      folders: new Set<string>(),
      projects: new Set<string>(),
    };
    findings.forEach((f: any) => {
      const { org, folder, proj } = extractHierarchy(f.resource_name);
      if (org !== 'Unknown') sets.orgs.add(org);
      if (folder !== 'Unknown') sets.folders.add(folder);
      if (proj !== 'Unknown') sets.projects.add(proj);
    });
    return {
      orgs: ['All', ...Array.from(sets.orgs)],
      folders: ['All', ...Array.from(sets.folders)],
      projects: ['All', ...Array.from(sets.projects)],
    };
  }, [findings]);
  
  const handleFilterChange = (event: SelectChangeEvent<string>) => {
    setFilters((prev: any) => ({ ...prev, [event.target.name]: event.target.value }));
  };

  return (
    <Paper sx={{ p: 2 }}>
      <Grid container spacing={2} alignItems="center">
        <Grid item><Typography variant="subtitle1">Filters</Typography></Grid>
        <Grid item xs>
          <FormControl size="small" fullWidth>
            <InputLabel>Organization</InputLabel>
            <Select name="org" value={filters.org} label="Organization" onChange={handleFilterChange}>
              {orgs.map(o => <MenuItem key={o} value={o}>{o}</MenuItem>)}
            </Select>
          </FormControl>
        </Grid>
        <Grid item xs>
          <FormControl size="small" fullWidth>
            <InputLabel>Folder</InputLabel>
            <Select name="folder" value={filters.folder} label="Folder" onChange={handleFilterChange}>
              {folders.map(f => <MenuItem key={f} value={f}>{f}</MenuItem>)}
            </Select>
          </FormControl>
        </Grid>
        <Grid item xs>
          <FormControl size="small" fullWidth>
            <InputLabel>Project</InputLabel>
            <Select name="project" value={filters.project} label="Project" onChange={handleFilterChange}>
              {projects.map(p => <MenuItem key={p} value={p}>{p}</MenuItem>)}
            </Select>
          </FormControl>
        </Grid>
        <Grid item xs>
          <FormControl size="small" fullWidth>
            <InputLabel>Regulation</InputLabel>
            <Select name="regulation" value={filters.regulation} label="Regulation" onChange={handleFilterChange}>
              {['All', 'CRA', 'DORA'].map(r => <MenuItem key={r} value={r}>{r}</MenuItem>)}
            </Select>
          </FormControl>
        </Grid>
      </Grid>
    </Paper>
  );
}

function ComplianceChart({ findings }: any) {
  const theme = useTheme();
  const { compliant, nonCompliant, other } = useMemo(() => {
    let c = 0, nc = 0;
    findings.forEach((f: any) => {
      if (isCompliant(f.status)) c++;
      else if (isNonCompliant(f.status)) nc++;
    });
    return { compliant: c, nonCompliant: nc, other: findings.length - c - nc };
  }, [findings]);
  
  const chartData = {
    labels: ['Conformant', 'Non-Conformant', 'Other'],
    datasets: [{
      data: [compliant, nonCompliant, other],
      backgroundColor: [
        theme.palette.success.light,
        theme.palette.error.light,
        theme.palette.divider,
      ],
      borderColor: [
        theme.palette.success.main,
        theme.palette.error.main,
        theme.palette.text.secondary,
      ],
      borderWidth: 1,
    }],
  };

  return (
    <Paper sx={{ p: 2, height: '100%', display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
      <Typography variant="h6" gutterBottom>Compliance Overview</Typography>
      <Box sx={{ flexGrow: 1, width: '100%', minHeight: 200 }}>
        <Doughnut data={chartData} options={{ maintainAspectRatio: false, plugins: { legend: { position: 'bottom' } } }} />
      </Box>
      <Stack direction="row" spacing={2} sx={{ mt: 2 }}>
        <Chip icon={<CheckCircle />} label={`Pass: ${compliant}`} color="success" variant="outlined" />
        <Chip icon={<ErrorIcon />} label={`Fail: ${nonCompliant}`} color="error" variant="outlined" />
      </Stack>
    </Paper>
  );
}


function ActionToolbar({ findings }: { findings: any[] }) {
  const handleShare = () => {
    navigator.clipboard.writeText(window.location.href);
    alert("URL copied to clipboard!");
  };

  const handleExportCSV = () => {
    const headers = ['Resource', 'Regulation', 'Status', 'Details'];
    const csvContent = [
      headers.join(','),
      ...findings.map((f: any) => `"${f.resource_name}","${f.regulation}","${f.status}","${f.details.replace(/"/g, '""')}"`)
    ].join('\n');
    const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.setAttribute('download', 'findings.csv');
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  };

  return (
    <Paper sx={{ p: 2, height: '100%', display: 'flex', alignItems: 'center', justifyContent: 'flex-end', gap: 2 }}>
      <Button startIcon={<Share />} onClick={handleShare} variant="outlined">Share</Button>
      <Button startIcon={<Download />} onClick={handleExportCSV} variant="contained" color="primary">Export CSV</Button>
    </Paper>
  );
}

function FindingsTable({ findings }: { findings: any[] }) {
  const safeFindings = Array.isArray(findings) ? findings : [];

  if (safeFindings.length === 0) {
    return (
      <Paper sx={{ p: 3, textAlign: 'center' }}>
        <Typography variant="body2" color="textSecondary">
          No findings available.
        </Typography>
      </Paper>
    );
  }

  return (
    <Box sx={{ flexGrow: 1, display: 'flex', flexDirection: 'column', gap: 2 }}>
      {safeFindings.map((finding, idx) => {
        const status = finding?.status || 'Unknown';
        const statusColor = isCompliant(status) ? "success" : isNonCompliant(status) ? "error" : "default";

        return (
          <Card key={idx} variant="outlined" sx={{ width: '100%', display: 'flex', flexDirection: 'column' }}>
            <CardContent>
              <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', flexWrap: 'wrap', gap: 1, mb: 1 }}>
                <Box>
                  <Typography variant="subtitle1" fontWeight="bold">
                    {finding?.resource_name || 'N/A'}
                  </Typography>
                  <Typography variant="caption" color="textSecondary">
                    Regulation: {finding?.regulation || 'N/A'}
                  </Typography>
                </Box>
                <Chip
                  label={status}
                  color={statusColor}
                  size="small"
                  variant="outlined"
                  sx={{ fontWeight: 'bold' }}
                />
              </Box>
              <Divider sx={{ my: 1 }} />
              <Typography 
                variant="body2" 
                color="textPrimary" 
                sx={{ 
                  whiteSpace: 'pre-wrap', 
                  wordBreak: 'break-word',
                  fontFamily: 'monospace',
                  backgroundColor: 'grey.50',
                  p: 1.5,
                  borderRadius: 1,
                  maxHeight: '300px',
                  overflowY: 'auto'
                }}
              >
                {finding?.details || 'No details provided.'}
              </Typography>
            </CardContent>
          </Card>
        );
      })}
    </Box>
  );
}

function ScanControls({ scope, setScope, regulation, setRegulation, loading, handleScan }: any) {
  return (
    <Paper sx={{ p: 2, display: 'flex', gap: 2, alignItems: 'center' }}>
      <TextField label="Scope" variant="outlined" size="small" value={scope} onChange={(e) => setScope(e.target.value)} fullWidth />
      <FormControl size="small" sx={{ minWidth: 120 }}>
        <InputLabel>Regulation</InputLabel>
        <Select value={regulation} label="Regulation" onChange={(e) => setRegulation(e.target.value)}>
          <MenuItem value="CRA">CRA</MenuItem>
          <MenuItem value="DORA">DORA</MenuItem>
        </Select>
      </FormControl>
      <Button variant="contained" color="primary" onClick={handleScan} disabled={loading} startIcon={loading ? <CircularProgress size={20} /> : <PlayArrow />}>
        Scan
      </Button>
    </Paper>
  );
}

function AgentLog({ events }: { events: any[] }) {
  return (
    <Paper sx={{ p: 2, height: '100%', display: 'flex', flexDirection: 'column' }}>
      <Typography variant="h6" gutterBottom>Agent Log</Typography>
      <Box sx={{ flexGrow: 1, overflowY: 'auto', maxHeight: 400 }}>
        {events.length === 0 ? (
          <Typography variant="body2" color="textSecondary">No events yet.</Typography>
        ) : (
          events.map((e, idx) => (
            <Box key={idx} sx={{ mb: 1, p: 1, bgcolor: 'grey.100', borderRadius: 1 }}>
              <Typography variant="caption" color="textSecondary">
                [{new Date(e.timestamp).toLocaleTimeString()}] {e.agent_name}
              </Typography>
              <Typography variant="body2" sx={{ whiteSpace: 'pre-wrap', wordBreak: 'break-word', mt: 0.5 }}>
                {e.details}
              </Typography>
            </Box>
          ))
        )}
      </Box>
    </Paper>
  );
}

function ScanResults({ scanResult, loading, jobId }: any) {
  if (loading) {
    return (
      <Paper sx={{ p: 2, height: '100%', display: 'flex', justifyContent: 'center', alignItems: 'center' }}>
        <CircularProgress />
      </Paper>
    );
  }

  if (!scanResult) {
    return (
      <Paper sx={{ p: 2, height: '100%', display: 'flex', justifyContent: 'center', alignItems: 'center' }}>
        <Typography color="textSecondary">No scan results to display. Run a scan to begin.</Typography>
      </Paper>
    );
  }

  return (
    <Paper sx={{ p: 2, height: '100%' }}>
      <Typography variant="h6" gutterBottom>Scan Results {jobId && `(Job ID: ${jobId})`}</Typography>
      <Typography variant="subtitle2" color="textSecondary" gutterBottom>Status: {scanResult.status}</Typography>
      <FindingsTable findings={scanResult.findings || []} />
    </Paper>
  );
}
