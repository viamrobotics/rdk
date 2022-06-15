// eslint-disable-next-line @typescript-eslint/no-var-requires
const colors = require('tailwindcss/colors');
// eslint-disable-next-line @typescript-eslint/no-var-requires
const { borderRadius, fontWeight } = require('tailwindcss/defaultTheme');
// eslint-disable-next-line @typescript-eslint/no-var-requires
const plugin = require('tailwindcss/plugin');

module.exports = {
  content: ['./index.html', './src/**/*.{vue,js,ts,jsx,tsx}'],
  theme: {
    screens: {
      sm: '480px',
      md: '768px',
      lg: '976px',
      xl: '1440px',
    },
    colors: {
      primary: colors.white,
      success: {
        50: '#f0fdf4',
        100: '#dcfce7',
        200: '#bbf7d0',
        300: '#86efac',
        400: '#4ade80',
        500: '#397F48',
        600: '#16a34a',
        700: '#15803d',
        800: '#166534',
        900: '#14532d',
      },
      warning: colors.orange,
      danger: {
        50: '#fef2f2',
        100: '#fee2e2',
        200: '#fecaca',
        300: '#fca5a5',
        400: '#f87171',
        500: '#BE3026',
        600: '#dc2626',
        700: '#b91c1c',
        800: '#991b1b',
        900: '#7f1d1d',
      },

      transparent: 'transparent',
      current: 'currentColor',

      black: '#18181b',
      white: '#fff',

      blue: colors.sky,
      cyan: colors.cyan,
      gray: colors.gray,
      green: colors.green,
      indigo: colors.indigo,
      orange: colors.orange,
      pink: colors.pink,
      purple: colors.purple,
      red: colors.rose,
      teal: colors.teal,
      yellow: colors.amber,
    },
    extend: {
      borderRadius: {
        button: borderRadius.lg,
        form: borderRadius.md,
      },
      fontFamily: {
        sans: ['Space Mono', 'sans-serif'],
        serif: ['Space Mono', 'serif'],
      },
      fontWeight: {
        button: fontWeight.medium,
        header: fontWeight.medium,
        label: fontWeight.medium,
      },
    },
  },
  variants: {
    padding: ['responsive', 'last'],
  },
  plugins: [
    plugin(({ addUtilities }) => {
      const animationDelay = {
        '.animation-delay-75': {
          'animation-delay': '75ms',
        },
        '.animation-delay-100': {
          'animation-delay': '100ms',
        },
        '.animation-delay-150': {
          'animation-delay': '150ms',
        },
        '.animation-delay-200': {
          'animation-delay': '200ms',
        },
        '.animation-delay-300': {
          'animation-delay': '300ms',
        },
        '.animation-delay-500': {
          'animation-delay': '500ms',
        },
        '.animation-delay-600': {
          'animation-delay': '600ms',
        },
        '.animation-delay-700': {
          'animation-delay': '700ms',
        },
        '.animation-delay-1000': {
          'animation-delay': '1000ms',
        },
      };

      addUtilities(animationDelay, ['responsive']);
    }),
    ({ addBase, config }) => {
      addBase({
        html: {
          color: config('theme.colors.gray.900'),
          backgroundColor: config('theme.colors.white'),
        },
        'html.dark': {
          color: config('theme.colors.white'),
          backgroundColor: config('theme.colors.gray.900'),
        },
      });
    },
  ],
};
