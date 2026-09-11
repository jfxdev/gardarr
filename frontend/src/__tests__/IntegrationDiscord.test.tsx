import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import IntegrationDiscordPage from '@/IntegrationDiscord'

const { list, create, update, remove, testDelivery, navigate, toast } = vi.hoisted(() => ({ list: vi.fn(), create: vi.fn(), update: vi.fn(), remove: vi.fn(), testDelivery: vi.fn(), navigate: vi.fn(), toast: { error: vi.fn(), success: vi.fn() } }))

vi.mock('@/services/discord', () => ({ discordService: { list, create, update, remove, test: testDelivery, history: vi.fn() } }))
vi.mock('react-router', () => ({ useNavigate: () => navigate }))
vi.mock('sonner', () => ({ toast }))

const destination = { uuid: 'discord-1', name: 'Alerts', webhook_configured: true, enabled: true, all_events: false, event_types: ['report.transfer.daily'], status_filter: [], category_filter: [], name_terms: [], created_at: '', updated_at: '' }

describe('IntegrationDiscordPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    list.mockResolvedValue({ data: [] })
    create.mockResolvedValue({ data: destination })
    update.mockResolvedValue({ data: destination })
    remove.mockResolvedValue({ data: null })
    testDelivery.mockResolvedValue({ data: { success: true } })
    vi.stubGlobal('confirm', vi.fn(() => true))
  })

  it('creates a destination from the empty state', async () => {
    render(<IntegrationDiscordPage />)
    expect(await screen.findByText('No Discord destinations configured yet.')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Add destination' }))
    const fields = screen.getAllByRole('textbox')
    fireEvent.change(fields[0], { target: { value: 'Night alerts' } })
    fireEvent.change(screen.getByPlaceholderText('https://discord.com/api/webhooks/...'), { target: { value: 'https://discord.com/api/webhooks/123/token' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(create).toHaveBeenCalledWith(expect.objectContaining({ name: 'Night alerts', webhook_url: 'https://discord.com/api/webhooks/123/token' })))
    expect(toast.success).toHaveBeenCalledWith('Discord destination saved.')
  })

  it('tests, edits and removes an existing destination', async () => {
    list.mockResolvedValue({ data: [destination] })
    render(<IntegrationDiscordPage />)
    expect(await screen.findByText('Alerts')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Test' }))
    await waitFor(() => expect(testDelivery).toHaveBeenCalledWith('discord-1'))
    fireEvent.click(screen.getByRole('button', { name: 'Edit' }))
    expect(screen.getByPlaceholderText('Leave blank to keep the current URL')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
    await waitFor(() => expect(remove).toHaveBeenCalledWith('discord-1'))
  })
})
