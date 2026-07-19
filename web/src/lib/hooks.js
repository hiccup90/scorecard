import { useEffect } from 'react';

export function useToastTimer(toast, setToast, ms = 1800) {
  useEffect(() => {
    if (!toast) return undefined;
    const t = setTimeout(() => setToast(''), ms);
    return () => clearTimeout(t);
  }, [toast, setToast, ms]);
}
