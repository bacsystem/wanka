"use client";

import { Alerta } from "khipu-design-system/feedback/alerta";
import { Button } from "khipu-design-system/ui/button";

export default function Error({ reset }: { error: Error & { digest?: string }; reset: () => void }) {
  return (
    <Alerta
      tono="error"
      titulo="No pudimos cargar la página"
      accion={
        <Button variant="outline" onClick={reset}>
          Reintentar
        </Button>
      }
    >
      Inténtalo de nuevo en unos segundos.
    </Alerta>
  );
}
