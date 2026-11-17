"use client";

import { ErrorState } from "@/components/shared/states";

export default function Error({ error, reset }: { error: Error & { digest?: string }; reset: () => void }) {
  return <div className="flex flex-1 items-center justify-center"><ErrorState detail={`${error.message}${error.digest ? `\ndigest: ${error.digest}` : ""}`} onRetry={reset} /></div>;
}
