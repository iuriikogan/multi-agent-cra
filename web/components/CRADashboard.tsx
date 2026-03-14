/**
 * Rationale: Implements the UI/UX or domain logic for the Next.js frontend, adhering to
 * React functional component paradigms and Material UI design specifications.
 * Terminology: CRA Dashboard, SSR (Server-Side Rendering), Component.
 * Measurability: Enhances user interaction by providing responsive, accessible interfaces.
 */
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
  CircularProgress
} from '@mui/material';
import ShareIcon from '@mui/icons-material/Share';
import DownloadIcon from '@mui/icons-material/Download';
import { Chart as ChartJS, ArcElement, Tooltip, Legend } from 'chart.js';
import { Doughnut } from 'react-chartjs-2';

ChartJS.register(ArcElement, Tooltip, Legend);

interface Finding {
  resource_name: string;
  status: string;
  details: string;
}

export default function CRADashboard() {
  const [findings, setFindings] = useState<Finding[]>([]);
  const [loading, setLoading] = useState(true);
  
  // Filters for refining the findings display based on GCP resource hierarchy.
  const [orgFilter, setOrgFilter] = useState('All');
  const [folderFilter, setFolderFilter] = useState('All');
  const [projectFilter, setProjectFilter] = useState('All');

  // Effect to parse URL params for initial filter state, ensuring deep linking and filter persistence work.
  useEffect(() => {
    // Parse URL params for initial state
    if (typeof window !== 'undefined') {
      const params = new URLSearchParams(window.location.search);
      if (params.has('org')) setOrgFilter(params.get('org')!);
      if (params.has('folder')) setFolderFilter(params.get('folder')!);
      if (params.has('project')) setProjectFilter(params.get('project')!);
    }

    fetchFindings();
  }, []);

  // Update URL when filters change to enable bookmarking and sharing of specific filtered views.
  useEffect(() => {
    if (typeof window !== 'undefined') {
      const url = new URL(window.location.href);
      if (orgFilter !== 'All') url.searchParams.set('org', orgFilter);
      else url.searchParams.delete('org');
      
      if (folderFilter !== 'All') url.searchParams.set('folder', folderFilter);
      else url.searchParams.delete('folder');
      
      if (projectFilter !== 'All') url.searchParams.set('project', projectFilter);
      else url.searchParams.delete('project');

      window.history.replaceState({}, '', url.toString());
    }
  }, [orgFilter, folderFilter, projectFilter]);

  const fetchFindings = async () => {
    try {
      setLoading(true);
      const res = await fetch('/api/findings');
      if (res.ok) {
        let data = await res.json();
        if (!data) data = []; // Ensure findings is always an array to prevent rendering errors.
        setFindings(data);
      }
    } catch (err) {
      console.error("Failed to fetch findings", err);
    } finally {
      // Ensure loading is set to false even if there's an error to prevent the UI from freezing in a loading state.
      setLoading(false);
    }
  };

  // Helper to extract hierarchy parts from resource_name.
  // Assumes GCP resource name formats like: //cloudresourcemanager.googleapis.com/organizations/123/folders/456/projects/my-proj/...
  // or simpler forms like: organizations/123/folders/456/projects/my-proj/...
  // or even just projects/my-proj.
  const extractHierarchy = (resourceName: string) => {
    if (!resourceName) return { org: 'Unknown', folder: 'Unknown', proj: 'Unknown' };
    const parts = resourceName.split('/');
    let org = 'Unknown', folder = 'Unknown', proj = 'Unknown';
    
    // Support for standard GCP resource paths
    const orgIdx = parts.indexOf('organizations');
    if (orgIdx !== -1 && orgIdx + 1 < parts.length) org = parts[orgIdx + 1];
    
    const folderIdx = parts.indexOf('folders');
    if (folderIdx !== -1 && folderIdx + 1 < parts.length) folder = parts[folderIdx + 1];
    
    const projIdx = parts.indexOf('projects');
    if (projIdx !== -1 && projIdx + 1 < parts.length) proj = parts[projIdx + 1];
    
    // Fallback if the path is simpler like "projects/xyz" or just a name
    if (proj === 'Unknown') {
      if (resourceName.startsWith('projects/')) {
        proj = resourceName.split('/')[1];
      } else if (parts.length > 0) {
        // Just use the first part as a project guess if it looks like a project id
        proj = parts[0];
      }
    }

    return { org, folder, proj };
  };

  // Unique values for dropdowns to prevent redundant UI elements and ensure clean filtering options for the user.
  const orgs = ['All', ...Array.from(new Set(findings.map(f => extractHierarchy(f.resource_name).org).filter(o => o !== 'Unknown')))];
  const folders = ['All', ...Array.from(new Set(findings.map(f => extractHierarchy(f.resource_name).folder).filter(f => f !== 'Unknown')))];
  const projects = ['All', ...Array.from(new Set(findings.map(f => extractHierarchy(f.resource_name).proj).filter(p => p !== 'Unknown')))];

  // Filtered data based on the currently selected dropdown values.
  // Applies an AND logic: a finding must match ALL selected filters to be included in the results list.
  const filteredFindings = findings.filter(f => {
    const { org, folder, proj } = extractHierarchy(f.resource_name);
    if (orgFilter !== 'All' && org !== orgFilter) return false;
    if (folderFilter !== 'All' && folder !== folderFilter) return false;
    if (projectFilter !== 'All' && proj !== projectFilter) return false;
    return true;
  });

  // Chart data for the Doughnut chart, providing a high-level summary of compliance status.
  // The 'Other' category captures findings with statuses that are neither strictly compliant nor non-compliant.
  const compliantCount = filteredFindings.filter(f => {
    const s = f.status.toLowerCase();
    return s === 'compliant' || s === 'true' || s === 'approved';
  }).length;
  const nonCompliantCount = filteredFindings.filter(f => {
    const s = f.status.toLowerCase();
    return s === 'non-compliant' || s === 'false' || s === 'failed' || s === 'rejected';
  }).length;
  const otherCount = filteredFindings.length - compliantCount - nonCompliantCount;

  const chartData = {
    labels: ['Compliant', 'Non-Compliant', 'Other'],
    datasets: [
      {
        data: [compliantCount, nonCompliantCount, otherCount],
        backgroundColor: ['rgba(75, 192, 192, 0.6)', 'rgba(255, 99, 132, 0.6)', 'rgba(201, 203, 207, 0.6)'],
        borderColor: ['rgba(75, 192, 192, 1)', 'rgba(255, 99, 132, 1)', 'rgba(201, 203, 207, 1)'],
        borderWidth: 1,
      },
    ],
  };

  const handleShare = () => {
    navigator.clipboard.writeText(window.location.href);
    alert("URL copied to clipboard!"); // Provide immediate feedback to the user that the share link is ready to use.
  };

  const handleExportCSV = () => {
    const headers = ['Resource Name', 'Status', 'Details'];
    const csvContent = [
      headers.join(','),
      // Escape double quotes within details to prevent CSV parsing issues and ensure data integrity.
      ...filteredFindings.map(f => `"${f.resource_name}","${f.status}","${f.details.replace(/"/g, '""')}"`)].join('\n');

    // Create a Blob and URL for client-side download, which avoids extra server-side load and simplifies the export process.
    const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `cra-findings-export-${new Date().toISOString().split('T')[0]}.csv`;
    link.click();
    URL.revokeObjectURL(url);
  };

  if (loading) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', p: 5 }}>
        <CircularProgress />
      </Box>
    );
  }

  return (
    <Box sx={{ p: 3 }}>
      <Grid container spacing={3}>
        <Grid item xs={12} md={4}>
          <Paper sx={{ p: 2, textAlign: 'center' }}>
            <Typography variant="h6">Compliance Overview</Typography>
            <Box sx={{ mt: 2, height: 200, display: 'flex', justifyContent: 'center' }}>
              <Doughnut data={chartData} options={{ maintainAspectRatio: false }} />
            </Box>
          </Paper>
        </Grid>
        
        <Grid item xs={12} md={8}>
          <Paper sx={{ p: 2 }}>
            <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 2 }}>
              <Typography variant="h6">Findings</Typography>
              <Box>
                <Button startIcon={<ShareIcon />} onClick={handleShare} sx={{ mr: 1 }}>Share</Button>
                <Button startIcon={<DownloadIcon />} onClick={handleExportCSV}>Export CSV</Button>
              </Box>
            </Box>
            
            <Box sx={{ display: 'flex', gap: 2, mb: 3 }}>
              <FormControl size="small" sx={{ minWidth: 120 }}>
                <InputLabel>Organization</InputLabel>
                <Select value={orgFilter} label="Organization" onChange={(e) => setOrgFilter(e.target.value)}>
                  {orgs.map(o => <MenuItem key={o} value={o}>{o}</MenuItem>)}
                </Select>
              </FormControl>
              
              <FormControl size="small" sx={{ minWidth: 120 }}>
                <InputLabel>Folder</InputLabel>
                <Select value={folderFilter} label="Folder" onChange={(e) => setFolderFilter(e.target.value)}>
                  {folders.map(f => <MenuItem key={f} value={f}>{f}</MenuItem>)}
                </Select>
              </FormControl>

              <FormControl size="small" sx={{ minWidth: 120 }}>
                <InputLabel>Project</InputLabel>
                <Select value={projectFilter} label="Project" onChange={(e) => setProjectFilter(e.target.value)}>
                  {projects.map(p => <MenuItem key={p} value={p}>{p}</MenuItem>)}
                </Select>
              </FormControl>
            </Box>

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
                  {filteredFindings.map((f, i) => (
                    <TableRow key={i}>
                      <TableCell>{f.resource_name}</TableCell>
                      <TableCell>
                        <Chip 
                          label={f.status} 
                          color={(f.status.toLowerCase() === 'compliant' || f.status.toLowerCase() === 'true' || f.status.toLowerCase() === 'approved') ? 'success' :
                            (f.status.toLowerCase() === 'non-compliant' || f.status.toLowerCase() === 'false' || f.status.toLowerCase() === 'failed' || f.status.toLowerCase() === 'rejected') ? 'error' : 'default'} 
                          size="small" 
                        />
                      </TableCell>
                      <TableCell>{f.details}</TableCell>
                    </TableRow>
                  ))}
                  {filteredFindings.length === 0 && (
                    <TableRow>
                      <TableCell colSpan={3} align="center">No findings match the selected filters.</TableCell>
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