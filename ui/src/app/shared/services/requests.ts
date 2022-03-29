import * as path from 'path';
import * as superagent from 'superagent';
const superagentPromise = require('superagent-promise');
import {BehaviorSubject, Observable, Observer} from 'rxjs';
import {filter} from 'rxjs/operators';
import {EventMessage} from '../models';

type Callback = (data: any) => void;

declare class EventSource {
    public onopen: Callback;
    public onmessage: Callback;
    public onerror: Callback;
    public readyState: number;
    constructor(url: string);
    public close(): void;
}

enum ReadyState {
    CONNECTING = 0,
    OPEN = 1,
    CLOSED = 2,
    DONE = 4
}

const agent: superagent.SuperAgentStatic = superagentPromise(superagent, global.Promise);

let baseHRef = '/';

const onError = new BehaviorSubject<superagent.ResponseError>(null);

function toAbsURL(val: string): string {
    return path.join(baseHRef, val);
}

function apiRoot(): string {
    return toAbsURL('/api/v1');
}

function initHandlers(req: superagent.Request) {
    req.on('error', err => onError.next(err));
    return req;
}

export default {
    setBaseHRef(val: string) {
        baseHRef = val;
    },
    agent,
    toAbsURL,
    onError: onError.asObservable().pipe(filter(err => err != null)),
    get(url: string) {
        return initHandlers(agent.get(`${apiRoot()}${url}`));
    },

    post(url: string) {
        return initHandlers(agent.post(`${apiRoot()}${url}`));
    },

    put(url: string) {
        return initHandlers(agent.put(`${apiRoot()}${url}`));
    },

    patch(url: string) {
        return initHandlers(agent.patch(`${apiRoot()}${url}`));
    },

    delete(url: string) {
        return initHandlers(agent.del(`${apiRoot()}${url}`));
    },

    loadEventSource(url: string): Observable<string> {
        return Observable.create((observer: Observer<any>) => {
            let eventSource = new EventSource(`${apiRoot()}${url}`);
            eventSource.onmessage = msg => observer.next(msg.data);
            eventSource.onerror = e => () => {
                observer.error(e);
                onError.next(e);
            };

            // EventSource does not provide easy way to get notification when connection closed.
            // check readyState periodically instead.
            const interval = setInterval(() => {
                if (eventSource && eventSource.readyState === ReadyState.CLOSED) {
                    observer.error('connection got closed unexpectedly');
                }
            }, 500);
            return () => {
                clearInterval(interval);
                eventSource.close();
                eventSource = null;
            };
        });
    },

    loadEventSourceMap<T>(url: string, getKey: (arg0: T) => string, initialEvents?: Promise<T[]>): Observable<Map<string, T>> {
        const map = new Map<string, T>();

        return new Observable<Map<string, T>>((observer: Observer<Map<string, T>>) => {
            if (initialEvents) {
                initialEvents.then(events => {
                    for (const event of events) {
                        map.set(getKey(event), event);
                        observer.next(map);
                    }
                });
            }

            let eventSource = new EventSource(`${apiRoot()}${url}`);
            eventSource.onmessage = (msg: MessageEvent) => {
                const eventMsg = JSON.parse(msg.data).result as EventMessage<T>;
                if (eventMsg.type === 'DELETED') {
                    map.delete(getKey(eventMsg.event));
                } else {
                    map.set(getKey(eventMsg.event), eventMsg.event);
                }
                observer.next(map);
            };
            eventSource.onerror = e => () => {
                observer.error(e);
                onError.next(e);
            };

            // EventSource does not provide easy way to get notification when connection closed.
            // check readyState periodically instead.
            const interval = setInterval(() => {
                if (eventSource && eventSource.readyState === ReadyState.CLOSED) {
                    observer.error('connection got closed unexpectedly');
                }
            }, 500);
            return () => {
                clearInterval(interval);
                eventSource.close();
                eventSource = null;
            };
        });
    }
};
