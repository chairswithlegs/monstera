'use client';

import { getUser } from '@/lib/api/user';
import { useEffect, useState } from 'react';
import type { User } from '@/lib/api/user';
import { Alert, AlertDescription } from '@/components/ui/alert';

export default function HomePage() {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getUser()
      .then(setUser)
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load'))
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <div className="text-muted-foreground">Loading...</div>;
  if (error) {
    return (
      <Alert variant="destructive">
        <AlertDescription>{error}</AlertDescription>
      </Alert>
    );
  }
  if (!user) return <div className="text-muted-foreground">Loading...</div>;

  return (
    <div className="space-y-8">
      <h1 className="text-2xl font-semibold">Welcome, @{user.username}</h1>

      <section className="space-y-2">
        <h2 className="text-lg font-medium">What is Monstera?</h2>
        <p className="text-muted-foreground">
          Monstera is a federated social server built on the ActivityPub protocol.
          It is fully compatible with Mastodon and the broader fediverse, meaning you can follow
          and interact with people on any ActivityPub-speaking platform from your Monstera account.
        </p>
      </section>

      <section className="space-y-2">
        <h2 className="text-lg font-medium">What is this UI for?</h2>
        <p className="text-muted-foreground">
          This web interface is for managing your account settings — things like your profile,
          preferences, and server administration. For day-to-day use (posting, following people,
          reading timelines) you should connect a Mastodon-compatible client.
        </p>
      </section>

      <section className="space-y-3">
        <h2 className="text-lg font-medium">Recommended Clients</h2>
        <ul className="space-y-2">
          <li>
            <a
              href="https://elk.zone"
              target="_blank"
              rel="noopener noreferrer"
              className="font-medium hover:underline"
            >
              Elk
            </a>
            <span className="text-muted-foreground"> — modern web client</span>
          </li>
          <li>
            <a
              href="https://mastodon.social/about"
              target="_blank"
              rel="noopener noreferrer"
              className="font-medium hover:underline"
            >
              Mastodon
            </a>
            <span className="text-muted-foreground"> — official web app, works with any Mastodon-compatible server</span>
          </li>
          <li>
            <a
              href="https://tapbots.com/ivory"
              target="_blank"
              rel="noopener noreferrer"
              className="font-medium hover:underline"
            >
              Ivory
            </a>
            <span className="text-muted-foreground"> — polished iOS app by Tapbots</span>
          </li>
          <li>
            <a
              href="https://getmammoth.app"
              target="_blank"
              rel="noopener noreferrer"
              className="font-medium hover:underline"
            >
              Mammoth
            </a>
            <span className="text-muted-foreground"> — iOS app</span>
          </li>
          <li>
            <a
              href="https://tusky.app"
              target="_blank"
              rel="noopener noreferrer"
              className="font-medium hover:underline"
            >
              Tusky
            </a>
            <span className="text-muted-foreground"> — Android app</span>
          </li>
        </ul>
      </section>
    </div>
  );
}
