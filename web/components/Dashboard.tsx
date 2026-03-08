import { useState, useEffect } from 'react';
import { Chart as ChartJS, ArcElement, Tooltip, Legend } from 'chart.js';
import { Doughnut } from 'react-chartjs-2';
import { Grid, Paper, Typography } from '@mui/material';

ChartJS.register(ArcElement, Tooltip, Legend);

export default function Dashboard() {
  const [stats, setStats] = useState({ compliant: 15, nonCompliant: 5 });

  useEffect(() => {
    // In a real app, fetch from /api/stats
    // For now, using mock data
    const fetchStats = async () => {
      // const res = await fetch('/api/stats');
      // const data = await res.json();
      // setStats(data);
    };
    fetchStats();
  }, []);

  const data = {
    labels: ['Compliant', 'Non-Compliant'],
    datasets: [
      {
        data: [stats.compliant, stats.nonCompliant],
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
      <Grid item xs={12} md={6} lg={4}>
        <Paper
          sx={{
            p: 2,
            display: 'flex',
            flexDirection: 'column',
            height: 240,
          }}
        >
          <Typography component="h2" variant="h6" color="primary" gutterBottom>
            Status Distribution
          </Typography>
          <div style={{ position: 'relative', height: '100%', width: '100%' }}>
            <Doughnut data={data} options={{ maintainAspectRatio: false }} />
          </div>
        </Paper>
      </Grid>
      <Grid item xs={12} md={6} lg={8}>
        <Paper sx={{ p: 2, display: 'flex', flexDirection: 'column', height: 240 }}>
           <Typography component="h2" variant="h6" color="primary" gutterBottom>
            Recent Activity
          </Typography>
          <Typography variant="body2">
            No recent agent activity found.
          </Typography>
        </Paper>
      </Grid>
    </Grid>
  );
}
