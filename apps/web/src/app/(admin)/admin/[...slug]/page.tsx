import { notFound } from "next/navigation";

/**
 * Catch-all of the console: without it, an unknown /admin/* URL would match the tenant app's
 * `[...slug]` and render the tenant 404. Every URL that reaches this page is a console 404.
 */
export default function AdminUnknownRoute() {
  notFound();
}
