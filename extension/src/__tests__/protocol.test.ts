import { createDialRequest, parseIncomingMessage } from '../shared/protocol';

describe('protocol', () => {
  describe('createDialRequest', () => {
    it('should create a valid dial request', () => {
      const request = createDialRequest('13800138000');
      expect(request).toEqual({
        type: 'dial',
        phone_number: '13800138000',
      });
    });

    it('should include the correct type field', () => {
      const request = createDialRequest('15912345678');
      expect(request.type).toBe('dial');
      expect(request.phone_number).toBe('15912345678');
    });
  });

  describe('parseIncomingMessage', () => {
    it('should parse a dial_result message', () => {
      const json = JSON.stringify({
        type: 'dial_result',
        success: true,
        error: '',
      });
      const msg = parseIncomingMessage(json);
      expect(msg).not.toBeNull();
      expect(msg!.type).toBe('dial_result');
      if (msg!.type === 'dial_result') {
        expect(msg!.success).toBe(true);
        expect(msg!.error).toBe('');
      }
    });

    it('should parse a failed dial_result message', () => {
      const json = JSON.stringify({
        type: 'dial_result',
        success: false,
        error: 'No phone connected',
      });
      const msg = parseIncomingMessage(json);
      expect(msg).not.toBeNull();
      if (msg!.type === 'dial_result') {
        expect(msg!.success).toBe(false);
        expect(msg!.error).toBe('No phone connected');
      }
    });

    it('should parse a status message', () => {
      const json = JSON.stringify({
        type: 'status',
        connected: true,
        phones: [
          { device_id: 'abc123', device_name: 'Phone 1', online: true },
        ],
      });
      const msg = parseIncomingMessage(json);
      expect(msg).not.toBeNull();
      if (msg!.type === 'status') {
        expect(msg!.connected).toBe(true);
        expect(msg!.phones).toHaveLength(1);
        expect(msg!.phones[0].device_id).toBe('abc123');
      }
    });

    it('should return null for invalid JSON', () => {
      const msg = parseIncomingMessage('not valid json');
      expect(msg).toBeNull();
    });

    it('should return null for unknown message type', () => {
      const json = JSON.stringify({ type: 'unknown', data: 123 });
      const msg = parseIncomingMessage(json);
      expect(msg).toBeNull();
    });

    it('should return null for empty object', () => {
      const json = JSON.stringify({});
      const msg = parseIncomingMessage(json);
      expect(msg).toBeNull();
    });

    it('should handle serialization roundtrip', () => {
      const request = createDialRequest('13800138000');
      const json = JSON.stringify(request);
      const parsed = JSON.parse(json);
      expect(parsed.type).toBe('dial');
      expect(parsed.phone_number).toBe('13800138000');
    });
  });
});
