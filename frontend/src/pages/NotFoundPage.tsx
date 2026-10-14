import { Link } from "react-router";

import { EmptyState } from "../components/ui";

export function NotFoundPage() {
  return (
    <EmptyState
      title="Page not found"
      description="The page you're looking for doesn't exist."
      action={
        <Link to="/" className="text-sm font-medium text-blue-600 hover:text-blue-700">
          Go to the dashboard
        </Link>
      }
    />
  );
}
