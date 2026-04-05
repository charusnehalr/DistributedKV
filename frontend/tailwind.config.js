/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      colors: {
        primary:   { DEFAULT: '#3b82f6', dark: '#2563eb' },
        secondary: { DEFAULT: '#8b5cf6', dark: '#7c3aed' },
        success:   { DEFAULT: '#10b981', dark: '#059669' },
        warning:   { DEFAULT: '#f59e0b', dark: '#d97706' },
        danger:    { DEFAULT: '#ef4444', dark: '#dc2626' },
      },
    },
  },
  plugins: [],
}
