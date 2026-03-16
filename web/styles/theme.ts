import { createTheme, responsiveFontSizes } from '@mui/material/styles';

// Define the color palette using Google's Material Design 3 colors.
const palette = {
  primary: {
    main: '#4285F4', // Google Blue
    light: '#E8F0FE',
    dark: '#174EA6',
    contrastText: '#FFFFFF',
  },
  secondary: {
    main: '#34A853', // Google Green
    light: '#E6F4EA',
    dark: '#1B873D',
    contrastText: '#FFFFFF',
  },
  error: {
    main: '#EA4335', // Google Red
    light: '#FCE8E6',
    dark: '#B3261E',
    contrastText: '#FFFFFF',
  },
  warning: {
    main: '#FBBC04', // Google Yellow
    light: '#FEF7E0',
    dark: '#E29903',
    contrastText: '#202124',
  },
  success: {
    main: '#34A853',
    light: '#E6F4EA',
    dark: '#1B873D',
    contrastText: '#FFFFFF',
  },
  background: {
    default: '#F8F9FA',
    paper: '#FFFFFF',
  },
  text: {
    primary: '#202124', // Almost Black
    secondary: '#5F6368', // Medium Gray
    disabled: '#9AA0A6', // Light Gray
  },
  divider: '#DADCE0',
};

// Create the base theme.
let theme = createTheme({
  palette,
  typography: {
    fontFamily: [
      'Google Sans',
      'Roboto',
      '"Helvetica Neue"',
      'Arial',
      'sans-serif',
    ].join(','),
    h1: {
      fontSize: '2.5rem',
      fontWeight: 700,
      letterSpacing: '-0.5px',
    },
    h2: {
      fontSize: '2rem',
      fontWeight: 700,
      letterSpacing: '-0.25px',
    },
    h3: {
      fontSize: '1.75rem',
      fontWeight: 600,
    },
    h4: {
      fontSize: '1.5rem',
      fontWeight: 600,
    },
    h5: {
      fontSize: '1.25rem',
      fontWeight: 600,
    },
    h6: {
      fontSize: '1.1rem',
      fontWeight: 600,
    },
    subtitle1: {
      fontSize: '1rem',
      fontWeight: 500,
      color: palette.text.secondary,
    },
    body1: {
      fontSize: '1rem',
      lineHeight: 1.6,
    },
    body2: {
      fontSize: '0.875rem',
      lineHeight: 1.5,
    },
    button: {
      textTransform: 'none',
      fontWeight: 600,
      letterSpacing: '0.25px',
    },
    caption: {
      fontSize: '0.75rem',
      color: palette.text.secondary,
    },
  },
  shape: {
    borderRadius: 12,
  },
  components: {
    MuiCssBaseline: {
      styleOverrides: `
        @import url('https://fonts.googleapis.com/css2?family=Google+Sans:wght@400;500;700&display=swap');
      `,
    },
    MuiButton: {
      defaultProps: {
        disableElevation: true,
      },
      styleOverrides: {
        root: {
          padding: '10px 24px',
        },
        contained: {
          boxShadow: 'none',
          '&:hover': {
            boxShadow: '0 1px 3px 1px rgba(60,64,67,0.15)',
          },
        },
      },
    },
    MuiPaper: {
      styleOverrides: {
        root: {
          border: `1px solid ${palette.divider}`,
          boxShadow: '0 1px 2px 0 rgba(60,64,67,0.3)',
        },
      },
    },
    MuiAppBar: {
      styleOverrides: {
        root: {
          backgroundColor: palette.background.paper,
          color: palette.text.primary,
          borderBottom: `1px solid ${palette.divider}`,
          boxShadow: 'none',
        },
      },
    },
    MuiTab: {
      styleOverrides: {
        root: {
          fontWeight: 600,
        },
      },
    },
    MuiTableCell: {
      styleOverrides: {
        head: {
          fontWeight: 700,
          color: palette.text.secondary,
          backgroundColor: palette.background.default,
        },
      },
    },
    MuiChip: {
      styleOverrides: {
        root: {
          fontWeight: 600,
        },
      },
    },
    MuiTooltip: {
      styleOverrides: {
        tooltip: {
          backgroundColor: palette.text.primary,
          color: palette.background.paper,
          fontSize: '0.875rem',
          fontWeight: 500,
        },
        arrow: {
          color: palette.text.primary,
        },
      },
    },
  },
});

// Apply responsive font sizes.
theme = responsiveFontSizes(theme);

export default theme;
