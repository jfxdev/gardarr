export interface DiscordIntegration {
  uuid: string;
  name: string;
  webhook_configured: boolean;
  enabled: boolean;
  all_events: boolean;
  event_types: string[];
  status_filter: string[];
  category_filter: string[];
  name_terms: string[];
  created_at: string;
  updated_at: string;
}
export interface DiscordIntegrationInput {
  name: string;
  webhook_url?: string;
  enabled?: boolean;
  all_events?: boolean;
  event_types?: string[];
  status_filter?: string[];
  category_filter?: string[];
  name_terms?: string[];
}
export interface DiscordDeliveryHistory { uuid: string; discord_id: string; event_type: string; status_code: number; error: string; created_at: string }
