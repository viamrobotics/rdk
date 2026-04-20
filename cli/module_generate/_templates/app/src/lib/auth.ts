import type { Credential } from '@viamrobotics/sdk';
import { getCookie, setCookie } from 'typescript-cookie';

const DEFAULT_HOST = 'default-host';

export interface HostAndCredentials {
	host: string;
	credentials: Credential;
	machineId: string | null;
}

export function getHostAndCredentials(): HostAndCredentials {
	const host = getCookie('host');
	const apiKeyId = getCookie('api-key-id');
	const apiKeySecret = getCookie('api-key');
	if (host && apiKeyId && apiKeySecret) {
		return {
			host,
			credentials: {
				type: 'api-key',
				payload: apiKeySecret,
				authEntity: apiKeyId
			},
			machineId: null
		};
	}

	const parts = window.location.pathname.split('/');
	if (parts && parts.length >= 3 && parts[1] == 'machine') {
		const machineCookieKey = parts[2];
		const cookieData = getCookie(machineCookieKey);
		if (cookieData) {
			try {
				const parsed = JSON.parse(cookieData);
				const id = parsed?.apiKey?.id;
				const key = parsed?.apiKey?.key;
				const h = parsed?.hostname;
				const machineId = parsed?.machineId || null;
				if (h && id && key) {
					return {
						host: h,
						credentials: { type: 'api-key', payload: key, authEntity: id },
						machineId
					};
				}
			} catch {
				// Invalid cookie data
			}
		}
	}

	const savedInputCookie = getCookie(DEFAULT_HOST);
	if (savedInputCookie) {
		try {
			const { host, id: apiKeyId, key: apiKeySecret } = JSON.parse(savedInputCookie);
			if (host && apiKeyId && apiKeySecret) {
				return {
					host,
					credentials: { type: 'api-key', payload: apiKeySecret, authEntity: apiKeyId },
					machineId: null
				};
			}
		} catch {
			// Invalid cookie data
		}
	}

	return {
		host: '',
		credentials: { type: 'api-key', payload: '', authEntity: '' },
		machineId: null
	};
}

export function saveHostInfo(host: string, id: string, key: string) {
	setCookie(DEFAULT_HOST, JSON.stringify({ host, key, id }));
}
