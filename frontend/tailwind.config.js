/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      colors: {
        brand: {
          50: '#eef4ff',
          100: '#d9e6ff',
          500: '#3b6ef6',
          600: '#2f57d1',
          700: '#2745a8',
        },
      },
    },
  },
  plugins: [],
}
