window.tailwind = window.tailwind || {};
window.tailwind.config = {
  corePlugins: {
    preflight: false,
  },
  theme: {
    extend: {
      colors: {
        rf: {
          green: '#6366f1',
          greenDark: '#4f46e5',
          bg: '#030712',
          card: '#111827',
          border: '#1f2937',
          text: '#f3f4f6',
          muted: '#9ca3af',
          subtle: '#6b7280',
          red: '#ef4444',
          blue: '#3b82f6',
          orange: '#f59e0b',
          purple: '#8b5cf6',
          cyan: '#06b6d4',
          pink: '#ec4899',
        },
      },
      boxShadow: {
        rf1: '0 4px 12px rgba(0,0,0,0.25)',
        rf2: '0 8px 24px rgba(0,0,0,0.35)',
        rf3: '0 20px 60px rgba(0,0,0,0.5)',
      },
      borderRadius: {
        rf: '8px',
        'rf-lg': '10px',
        'rf-xl': '12px',
      },
      keyframes: {
        'slide-up-fade': {
          '0%': { transform: 'translateY(14px)', opacity: '0' },
          '100%': { transform: 'translateY(0)', opacity: '1' },
        },
        'slide-in-left': {
          '0%': { transform: 'translateX(-100%)', opacity: '0' },
          '100%': { transform: 'translateX(0)', opacity: '1' },
        },
        'fade-in': {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' },
        },
      },
      animation: {
        'slide-up-fade': 'slide-up-fade .25s ease',
        'slide-in-left': 'slide-in-left .25s ease',
        'fade-in': 'fade-in .2s ease',
      },
    },
  },
};
