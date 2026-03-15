import React, { useState, useEffect } from 'react';
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
  Stack
} from '@mui/material';
import ShareIcon from '@mui/icons-material/Share';
import DownloadIcon from '@mui/icons-material/Download';
import { Chart as ChartJS, ArcElement, Tooltip, Legend } from 'chart.js';
import { Doughnut } from 'react-chartjs-2';

ChartJS.register(ArcElement, Tooltip, Legend);

// Finding represents a single conformity assessment result for a target PDE.
interface Finding {
  details: string;       // Detailed description of the assessment result
  regulation: string;    // Regulation framework (CRA or DORA)
}

// CRADashboard provides the main UI for visualizing and filtering conformity findings.
export default function CRADashboard() {
  const [findings, setFindings] = useState<Finding[]>([]);
  const [loading, setLoading] = useState(true);

  const [orgFilter, setOrgFilter] = useState('All');
  const [folderFilter, setFolderFilter] = useState('All');
  const [projectFilter, setProjectFilter] = useState('All');
  const [regulationFilter, setRegulationFilter] = useState('All');

  useEffect(() => {
    // Parse URL params for initial state to support direct linking.
    if (typeof window !== 'undefined') {
      const params = new URLSearchParams(window.location.search);
      if (params.has('org')) setOrgFilter(params.get('org')!);
      if (params.has('folder')) setFolderFilter(params.get('folder')!);
      if (params.has('project')) setProjectFilter(params.get('project')!);
      if (params.has('regulation')) setRegulationFilter(params.get('regulation')!);
    }

    fetchFindings();
  }, []);

  useEffect(() => {
    // Sync filter state with the browser URL for consistency.
    if (typeof window !== 'undefined') {
      const url = new URL(window.location.href);
      if (orgFilter !== 'All') url.searchParams.set('org', orgFilter);
      else url.searchParams.delete('org');
      
      if (folderFilter !== 'All') url.searchParams.set('folder', folderFilter);
      else url.searchParams.delete('folder');
      
      if (projectFilter !== 'All') url.searchParams.set('project', projectFilter);
      else url.searchParams.delete('project');

      if (regulationFilter !== 'All') url.searchParams.set('regulation', regulationFilter);
      else url.searchParams.delete('regulation');

      window.history.replaceState({}, '', url.toString());
    }
  }, [orgFilter, folderFilter, projectFilter, regulationFilter]);

  // fetchFindings retrieves the full list of findings from the backend API.
  const fetchFindings = async () => {
    try {
      setLoading(true);
      const res = await fetch('/api/findings');
      if (res.ok) {
        let data = await res.json();
        if (!data) data = [];
        setFindings(data);
      }
    } catch (err) {
      console.error("Failed to fetch findings", err);
    } finally {
      setLoading(false);
    }
  };

  // extractHierarchy parses a GCP resource path into its component parts.
  const extractHierarchy = (resourceName: string) => {
    if (!resourceName) return { org: 'Unknown', folder: 'Unknown', proj: 'Unknown' };
    const parts = resourceName.split('/');
    let org = 'Unknown', folder = 'Unknown', proj = 'Unknown';

    const orgIdx = parts.indexOf('organizations');
    if (orgIdx !== -1 && orgIdx + 1 < parts.length) org = parts[orgIdx + 1];
    
    const folderIdx = parts.indexOf('folders');
    if (folderIdx !== -1 && folderIdx + 1 < parts.length) folder = parts[folderIdx + 1];
    
    const projIdx = parts.indexOf('projects');
    if (projIdx !== -1 && projIdx + 1 < parts.length) proj = parts[projIdx + 1];

    if (proj === 'Unknown') {
      if (resourceName.startsWith('projects/')) {
        proj = resourceName.split('/')[1];
      } else if (parts.length > 0) {
        proj = parts[0];
      }
    }

    return { org, folder, proj };
  };

  const orgs = ['All', ...Array.from(new Set(findings.map(f => extractHierarchy(f.resource_name).org).filter(o => o !== 'Unknown')))];
  const folders = ['All', ...Array.from(new Set(findings.map(f => extractHierarchy(f.resource_name).folder).filter(f => f !== 'Unknown')))];
  const projects = ['All', ...Array.from(new Set(findings.map(f => extractHierarchy(f.resource_name).proj).filter(p => p !== 'Unknown')))];
  const regulations = ['All', 'CRA', 'DORA'];

  const filteredFindings = findings.filter(f => {
    const { org, folder, proj } = extractHierarchy(f.resource_name);
    if (orgFilter !== 'All' && org !== orgFilter) return false;
    if (folderFilter !== 'All' && folder !== folderFilter) return false;
    if (projectFilter !== 'All' && proj !== projectFilter) return false;
    if (regulationFilter !== 'All' && f.regulation !== regulationFilter) return false;
    return true;
  });

  const compliantCount = filteredFindings.filter(f => {
    const s = f.status.toLowerCase();
    return s === 'compliant' || s === 'true' || s === 'approved' || s === 'conformant';
  }).length;
  const nonCompliantCount = filteredFindings.filter(f => {
    const s = f.status.toLowerCase();
    return s === 'non-compliant' || s === 'false' || s === 'failed' || s === 'rejected' || s === 'non-conformant';
  }).length;
  const otherCount = filteredFindings.length - compliantCount - nonCompliantCount;

  const chartData = {
    labels: ['Conformant', 'Non-Conformant', 'Other'],
    datasets: [
      {
        data: [compliantCount, nonCompliantCount, otherCount],
        backgroundColor: ['#34a85399', '#d9302599', '#dadce099'],
        borderColor: ['#34a853', '#d93025', '#dadce0'],
        borderWidth: 1,
      },
    ],
  };

  // handleShare copies the current filtered dashboard URL to the clipboard.
  const handleShare = () => {
    navigator.clipboard.writeText(window.location.href);
    alert("URL copied to clipboard!"); 
  };

  // handleExportCSV generates and initiates a CSV download of the filtered results.
  const handleExportCSV = () => {
    const headers = ['Target PDE', 'Regulation', 'Conformity Status', 'Assessment Details'];
    const csvContent = [
      headers.join(','),
      ...filteredFindings.map(f => `"${f.resource_name}","${f.regulation}","${f.status}","${f.details.replace(/"/g, '""')}"`)].join('\n');

    const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `compliance-findings-export-${new Date().toISOString().split('T')[0]}.csv`;
    link.click();
    URL.revokeObjectURL(url);
  };

  if (loading) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', p: 10 }}>
        <CircularProgress />
      </Box>
    );
  }

  return (
    <Box>
      <Grid container spacing={4}>
        <Grid item xs={12} md={4}>
          <Paper sx={{ p: 4, height: '100%', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center' }}>
            <Typography variant="h6" gutterBottom>Compliance Overview</Typography>
            <Box sx={{ mt: 2, height: 240, width: '100%', display: 'flex', justifyContent: 'center' }}>
              <Doughnut data={chartData} options={{ maintainAspectRatio: false }} />
            </Box>
            <Stack direction="row" spacing={2} sx={{ mt: 3, width: '100%', justifyContent: 'center' }}>
              <Box sx={{ textAlign: 'center' }}>
                <Typography variant="h5" color="secondary.main">{compliantCount}</Typography>
                <Typography variant="caption" color="text.secondary">Pass</Typography>
              </Box>
              <Box sx={{ textAlign: 'center' }}>
                <Typography variant="h5" color="error.main">{nonCompliantCount}</Typography>
                <Typography variant="caption" color="text.secondary">Fail</Typography>
              </Box>
            </Stack>
          </Paper>
        </Grid>
        
        <Grid item xs={12} md={8}>
          <Paper sx={{ p: 4 }}>
            <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', mb: 4 }}>
              <Box>
                <Typography variant="h6">Vulnerability & Conformity Findings</Typography>
                <Typography variant="body2" color="text.secondary">Detailed assessment results per resource</Typography>
              </Box>
              <Box>
                <Button variant="outlined" startIcon={<ShareIcon />} onClick={handleShare} sx={{ mr: 1 }} size="small">Share</Button>
                <Button variant="outlined" startIcon={<DownloadIcon />} onClick={handleExportCSV} size="small">Export</Button>
              </Box>
            </Box>
            
            <Box sx={{ display: 'flex', gap: 2, mb: 4 }}>
              <FormControl size="small" sx={{ minWidth: 160 }}>
                <InputLabel>Organization</InputLabel>
                <Select value={orgFilter} label="Organization" onChange={(e: SelectChangeEvent) => setOrgFilter(e.target.value)}>
                  {orgs.map(o => <MenuItem key={o} value={o}>{o}</MenuItem>)}
                </Select>
              </FormControl>
              
              <FormControl size="small" sx={{ minWidth: 160 }}>
                <InputLabel>Folder</InputLabel>
                <Select value={folderFilter} label="Folder" onChange={(e: SelectChangeEvent) => setFolderFilter(e.target.value)}>
                  {folders.map(f => <MenuItem key={f} value={f}>{f}</MenuItem>)}
                </Select>
              </FormControl>

              <FormControl size="small" sx={{ minWidth: 160 }}>
                <InputLabel>Project</InputLabel>
                <Select value={projectFilter} label="Project" onChange={(e: SelectChangeEvent) => setProjectFilter(e.target.value)}>
                  {projects.map(p => <MenuItem key={p} value={p}>{p}</MenuItem>)}
                </Select>
              </FormControl>

              <FormControl size="small" sx={{ minWidth: 160 }}>
                <InputLabel>Regulation</InputLabel>
                <Select value={regulationFilter} label="Regulation" onChange={(e: SelectChangeEvent) => setRegulationFilter(e.target.value)}>
                  {regulations.map(r => <MenuItem key={r} value={r}>{r}</MenuItem>)}
                </Select>
              </FormControl>
            </Box>

            <TableContainer>
              <Table size="medium">
                <TableHead>
                  <TableRow>
                    <TableCell>Resource</TableCell>
                    <TableCell align="center">Framework</TableCell>
                    <TableCell align="center">Status</TableCell>
                    <TableCell>Details</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {filteredFindings.map((f, i) => (
                    <TableRow key={i} hover>
                      <TableCell sx={{ fontFamily: 'monospace', fontSize: '0.85rem' }}>{f.resource_name.split('/').pop()}</TableCell>
                      <TableCell align="center">
                        <Chip label={f.regulation} size="small" variant="outlined" sx={{ fontWeight: 500 }} />
                      </TableCell>
                      <TableCell align="center">
                        <Chip 
                          label={f.status} 
                          variant="outlined"
                          color={(f.status.toLowerCase() === 'compliant' || f.status.toLowerCase() === 'true' || f.status.toLowerCase() === 'approved' || f.status.toLowerCase() === 'conformant') ? 'success' :
                            (f.status.toLowerCase() === 'non-compliant' || f.status.toLowerCase() === 'false' || f.status.toLowerCase() === 'failed' || f.status.toLowerCase() === 'rejected' || f.status.toLowerCase() === 'non-conformant') ? 'error' : 'default'}
                          size="small"
                          sx={{ fontWeight: 600 }}
                        />
                      </TableCell>
                      <TableCell sx={{ fontSize: '0.9rem' }}>{f.details}</TableCell>
                    </TableRow>
                  ))}
                  {filteredFindings.length === 0 && (
                    <TableRow>
                      <TableCell colSpan={4} align="center" sx={{ py: 4, color: 'text.secondary' }}>No findings match the selected filters.</TableCell>
                    </TableRow>
                  )}
                </TableBody>
              </Table>
            </TableContainer>
          </Paper>
        </Grid>
      </Grid>
    </Box>
  );
}