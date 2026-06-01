import { useState, useEffect, useCallback, useRef } from "react";

// Detect Chinese mobile numbers (11 digits starting with 1)
function extractPhoneNumber(text: string): string | null {
  const match = text.match(/1\d{10}/);
  return match ? match[0] : null;
}

interface ClipboardHook {
  detectedNumber: string | null;
  clearDetected: () => void;
}

export function useClipboard(): ClipboardHook {
  const [detectedNumber, setDetectedNumber] = useState<string | null>(null);
  const lastClipboardRef = useRef<string>("");

  const clearDetected = useCallback(() => {
    setDetectedNumber(null);
  }, []);

  useEffect(() => {
    const checkClipboard = async () => {
      try {
        // Use browser Clipboard API when available
        if (navigator.clipboard && navigator.clipboard.readText) {
          const text = await navigator.clipboard.readText();
          if (text !== lastClipboardRef.current) {
            lastClipboardRef.current = text;
            const phone = extractPhoneNumber(text);
            if (phone) {
              setDetectedNumber(phone);
            }
          }
        }
      } catch {
        // Clipboard access may be denied - silently ignore
      }
    };

    const interval = setInterval(checkClipboard, 2000);
    return () => clearInterval(interval);
  }, []);

  return { detectedNumber, clearDetected };
}
