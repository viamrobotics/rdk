import type { Config } from 'tailwindcss';
import { theme } from '@viamrobotics/prime-core/theme';

export default {
  content: [
    './src/**/*.{html,svelte,js,ts}',
    './node_modules/@viamrobotics/prime/dist/prime.js',
    './node_modules/@viamrobotics/prime-core/dist/**/*.svelte',
    './node_modules/@viamrobotics/prime-blocks/dist/**/*.svelte',
  ],
  theme: {
    extend: {
      ...theme.extend,
      fontFamily: {
        'space-grotesk': 'Space Grotesk',
        'public-sans': 'Public Sans, sans-serif',
      },
      boxShadow: {
        sm: '0 4px 32px rgba(0, 0, 0, 0.06)',
      },
      textColor: theme.extend.textColor,
      fill: {
        'subtle-1': '#4e4f52',
        'subtle-2': '#7a7c80',
      },
      borderColor: theme.extend.borderColor,
      backgroundColor: {
        1: '#FBFBFC',
        2: '#F7F7F8',
        3: '#F1F1F4',
        ...theme.extend.backgroundColor,
      },
      colors: {
        'danger-fg': '#be3536',
        'danger-bg': '#fcecea',
        'danger-border': '#edc0bf',
        'warning-fg': '#a6570f',
        'warning-bg': '#fef3e0',
        'warning-border': '#e9c89d',
        'success-fg': '#3d7d3f',
        'success-bg': '#e0fae3',
        'success-border': '#b9dcbc',
        'info-fg': '#0066cc',
        'info-bg': '#e1f3ff',
        'info-border': '#b6d1f4',
        'disabled-fg': '#9c9ca4',
        'disabled-bg': '#f2f2f4',
        'text-highlight': '#e2f1fd',
        ...theme.extend.colors,
      },
    },
  },
  plugins: [],
  safelist: ['list-disc', 'h-[400px]'],
} satisfies Config;
