import type { Config } from 'tailwindcss';

export default {
  content: ['./src/**/*.{html,vue,svelte,js,ts}'],
  theme: {
    extend: {
      fontFamily: {
        'space-grotesk': 'Space Grotesk',
        'public-sans': 'Public Sans, sans-serif',
      },
      boxShadow: {
        sm: '0 4px 32px rgba(0, 0, 0, 0.06)',
      },
      textColor: {
        heading: '#131414',
        default: '#282829',
        'subtle-1': '#4e4f52',
        'subtle-2': '#7a7c80',
        disabled: '#9c9ca4',
        link: '#0066CC',
      },
      fill: {
        'subtle-1': '#4e4f52',
        'subtle-2': '#7a7c80',
      },
      borderColor: {
        light: '#e4e4e6',
        medium: '#d7d7d9',
      },
      backgroundColor: {
        1: '#FBFBFC',
        2: '#F7F7F8',
        3: '#F1F1F4',
      },
      colors: {
        black: '#131414',
        'gray-9': '#282829',
        'gray-8': '#4e4f52',
        'gray-7': '#7a7c80',
        'gray-6': '#9c9ca4',
        'gray-5': '#c5c6cc',
        'gray-4': '#d7d7d9',
        'gray-3': '#e4e4e6',
        'gray-2': '#edeef0',
        'gray-1': '#f7f7f8',
        'danger-fg': '#be3536',
        'danger-bg': '#fcecea',
        'danger-border': '#edc0bf',
        'warning-fg': '#a6570f',
        'warning-bg': '#fef3e0',
        'warning-bright': '#ddab3f',
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
        'power-button': '#00EF83',
        'power-wire': '#FF0047',
      },
    },
  },
  plugins: [],
  safelist: ['list-disc', 'h-[400px]'],
} satisfies Config;
