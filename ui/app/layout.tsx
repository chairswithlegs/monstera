import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import "./globals.css";
import { IntlProvider } from "@/components/intl-provider";
import { Footer } from "@/components/footer";
import enMessages from "../messages/en.json";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "Monstera",
  description: "A self-hosted ActivityPub server",
};

const messages = { en: enMessages };
// To add a new locale: import frMessages from '../messages/fr.json' and add fr: frMessages

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html suppressHydrationWarning>
      <body
        className={`${geistSans.variable} ${geistMono.variable} antialiased flex flex-col min-h-screen`}
      >
        <IntlProvider messages={messages}>
          <div className="flex flex-col flex-1">
            {children}
          </div>
          <Footer />
        </IntlProvider>
      </body>
    </html>
  );
}
