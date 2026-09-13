import type { Metadata } from "next";
import { ForbiddenState } from "@/components/shared/states";

export const metadata: Metadata = { title: "Sin permisos" };

export default function ForbiddenPage() {
  return <div className="flex flex-1 items-center justify-center"><ForbiddenState /></div>;
}
