/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { SettingsPageProvider } from '../../components/settings-page-context'
import { ConcurrencySection } from '../concurrency-section'

const { mutateAsync } = vi.hoisted(() => ({ mutateAsync: vi.fn() }))
vi.mock('../../hooks/use-update-option', () => ({
  useUpdateOption: () => ({ mutateAsync, isPending: false }),
}))

function renderSettings(
  value = '{"enabled":true,"default_limit":3,"groups":{},"users":{}}'
) {
  const actions = document.createElement('div')
  document.body.appendChild(actions)
  const view = render(
    <SettingsPageProvider actionsContainer={actions}>
      <ConcurrencySection value={value} />
    </SettingsPageProvider>
  )
  return { ...view, actions }
}

describe('user concurrency settings', () => {
  beforeEach(() => {
    mutateAsync.mockResolvedValue({ success: true })
  })

  test('saves the default, group and user limits as one update', async () => {
    const view = renderSettings()
    fireEvent.change(
      screen.getByRole('spinbutton', {
        name: 'Default concurrent requests per user',
      }),
      { target: { value: '5' } }
    )
    fireEvent.input(
      screen.getByRole('textbox', { name: 'Concurrency by user group' }),
      { target: { value: '{"vip":10}' } }
    )
    fireEvent.input(
      screen.getByRole('textbox', { name: 'Concurrency by user ID' }),
      { target: { value: '{"123":0}' } }
    )
    fireEvent.click(
      screen.getByRole('button', { name: 'Save concurrency limits' })
    )
    await waitFor(() =>
      expect(mutateAsync).toHaveBeenCalledWith({
        key: 'UserConcurrencyLimit',
        value:
          '{"enabled":true,"default_limit":5,"groups":{"vip":10},"users":{"123":0}}',
      })
    )
    view.unmount()
    view.actions.remove()
  })

  test('rejects fractional limits and invalid user IDs before saving', async () => {
    const view = renderSettings()
    fireEvent.change(
      screen.getByRole('spinbutton', {
        name: 'Default concurrent requests per user',
      }),
      { target: { value: '1.5' } }
    )
    fireEvent.input(
      screen.getByRole('textbox', { name: 'Concurrency by user ID' }),
      { target: { value: '{"01":2}' } }
    )
    fireEvent.click(
      screen.getByRole('button', { name: 'Save concurrency limits' })
    )
    await waitFor(() =>
      expect(
        screen.getByRole('textbox', { name: 'Concurrency by user ID' })
      ).toHaveAttribute('aria-invalid', 'true')
    )
    expect(
      screen.getByRole('spinbutton', {
        name: 'Default concurrent requests per user',
      })
    ).toHaveAttribute('aria-invalid', 'true')
    expect(mutateAsync).not.toHaveBeenCalled()
    view.unmount()
    view.actions.remove()
  })

  test('keeps the edited limit after a failed save so it can be retried', async () => {
    mutateAsync.mockRejectedValueOnce(new Error('save failed'))
    const view = renderSettings()
    const input = screen.getByRole('spinbutton', {
      name: 'Default concurrent requests per user',
    })
    fireEvent.change(input, { target: { value: '7' } })
    fireEvent.click(
      screen.getByRole('button', { name: 'Save concurrency limits' })
    )
    await waitFor(() => expect(mutateAsync).toHaveBeenCalledTimes(1))
    expect(input).toHaveValue(7)
    fireEvent.click(
      screen.getByRole('button', { name: 'Save concurrency limits' })
    )
    await waitFor(() => expect(mutateAsync).toHaveBeenCalledTimes(2))
    expect(JSON.parse(mutateAsync.mock.calls[1][0].value).default_limit).toBe(7)
    view.unmount()
    view.actions.remove()
  })

  test('loads omitted overrides and allows explicitly disabling the limiter', async () => {
    const view = renderSettings('{"enabled":true,"default_limit":3}')
    fireEvent.click(
      screen.getByRole('switch', { name: 'Enable user concurrency limits' })
    )
    fireEvent.click(
      screen.getByRole('button', { name: 'Save concurrency limits' })
    )
    await waitFor(() =>
      expect(mutateAsync).toHaveBeenCalledWith({
        key: 'UserConcurrencyLimit',
        value: '{"enabled":false,"default_limit":3,"groups":{},"users":{}}',
      })
    )
    view.unmount()
    view.actions.remove()
  })
})
