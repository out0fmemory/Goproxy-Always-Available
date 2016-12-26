#define UNICODE
#define _UNICODE

#include <wchar.h>
#include <windows.h>
#include <wininet.h>
#include <shellapi.h>
#include <stdio.h>
#include <wininet.h>
#include <io.h>
#include <ras.h>
#include <raserror.h>
#include <psapi.h>
#include "resource.h"

// #pragma comment(lib, "rasapi32.lib")
// #pragma comment(lib, "shell32.lib")
// #pragma comment(lib, "psapi.lib")
// #pragma comment(lib, "advapi32.lib")
// #pragma comment(lib, "wininet.lib")

extern "C" WINBASEAPI HWND WINAPI GetConsoleWindow();

#define NID_UID 123
#define WM_TASKBARNOTIFY WM_USER+20
#define WM_TASKBARNOTIFY_MENUITEM_SHOW (WM_USER + 21)
#define WM_TASKBARNOTIFY_MENUITEM_HIDE (WM_USER + 22)
#define WM_TASKBARNOTIFY_MENUITEM_RELOAD (WM_USER + 23)
#define WM_TASKBARNOTIFY_MENUITEM_ABOUT (WM_USER + 24)
#define WM_TASKBARNOTIFY_MENUITEM_EXIT (WM_USER + 25)
#define WM_TASKBARNOTIFY_MENUITEM_PROXYLIST_BASE (WM_USER + 26)

HINSTANCE hInst;
HWND hWnd;
HWND hConsole;
WCHAR szTitle[64] = L"";
WCHAR szWindowClass[16] = L"taskbar";
WCHAR szCommandLine[1024] = L"";
WCHAR szTooltip[512] = L"";
WCHAR szBalloon[512] = L"";
WCHAR szEnvironment[1024] = L"";
WCHAR szProxyString[2048] = L"";
CHAR szRasPbk[4096] = "";
WCHAR *lpProxyList[8] = {0};
volatile DWORD dwChildrenPid;

static DWORD MyGetProcessId(HANDLE hProcess)
{
	// https://gist.github.com/kusma/268888
	typedef DWORD (WINAPI *pfnGPI)(HANDLE);
	typedef ULONG (WINAPI *pfnNTQIP)(HANDLE, ULONG, PVOID, ULONG, PULONG);

	static int first = 1;
	static pfnGPI pfnGetProcessId;
	static pfnNTQIP ZwQueryInformationProcess;
	if (first)
	{
		first = 0;
		pfnGetProcessId = (pfnGPI)GetProcAddress(
			GetModuleHandleW(L"KERNEL32.DLL"), "GetProcessId");
		if (!pfnGetProcessId)
			ZwQueryInformationProcess = (pfnNTQIP)GetProcAddress(
				GetModuleHandleW(L"NTDLL.DLL"),
				"ZwQueryInformationProcess");
	}
	if (pfnGetProcessId)
		return pfnGetProcessId(hProcess);
	if (ZwQueryInformationProcess)
	{
		struct
		{
			PVOID Reserved1;
			PVOID PebBaseAddress;
			PVOID Reserved2[2];
			ULONG UniqueProcessId;
			PVOID Reserved3;
		} pbi;
		ZwQueryInformationProcess(hProcess, 0, &pbi, sizeof(pbi), 0);
		return pbi.UniqueProcessId;
	}
	return 0;
}


static BOOL MyEndTask(DWORD pid)
{
	WCHAR szCmd[1024] = {0};
	wsprintf(szCmd, L"taskkill /f /pid %d", pid);
	return _wsystem(szCmd) == 0;
}

BOOL ShowTrayIcon(LPCTSTR lpszProxy, DWORD dwMessage=NIM_ADD)
{
	NOTIFYICONDATA nid;
	ZeroMemory(&nid, sizeof(NOTIFYICONDATA));
	nid.cbSize = (DWORD)sizeof(NOTIFYICONDATA);
	nid.hWnd   = hWnd;
	nid.uID	   = NID_UID;
	nid.uFlags = NIF_ICON|NIF_MESSAGE;
	nid.dwInfoFlags=NIIF_INFO;
	nid.uCallbackMessage = WM_TASKBARNOTIFY;
	nid.hIcon = LoadIcon(hInst, (LPCTSTR)IDI_SMALL);
	nid.uTimeout = 3 * 1000 | NOTIFYICON_VERSION;
	lstrcpy(nid.szInfoTitle, szTitle);
	if (lpszProxy)
	{
		nid.uFlags |= NIF_INFO|NIF_TIP;
		if (lstrlen(lpszProxy) > 0)
		{
			lstrcpy(nid.szTip, lpszProxy);
			lstrcpy(nid.szInfo, lpszProxy);
		}
		else
		{
			lstrcpy(nid.szInfo, szBalloon);
			lstrcpy(nid.szTip, szTooltip);
		}
	}
	Shell_NotifyIcon(dwMessage, &nid);
	return TRUE;
}

BOOL DeleteTrayIcon()
{
	NOTIFYICONDATA nid;
	nid.cbSize = (DWORD)sizeof(NOTIFYICONDATA);
	nid.hWnd   = hWnd;
	nid.uID	   = NID_UID;
	Shell_NotifyIcon(NIM_DELETE, &nid);
	return TRUE;
}


LPCTSTR GetWindowsProxy()
{
	static WCHAR szProxy[1024] = {0};
	HKEY hKey;
	DWORD dwData = 0;
	DWORD dwSize = sizeof(DWORD);

	if (ERROR_SUCCESS == RegOpenKeyEx(HKEY_CURRENT_USER,
									  L"Software\\Microsoft\\Windows\\CurrentVersion\\Internet Settings",
									  0,
									  KEY_READ | 0x0200,
									  &hKey))
	{
		szProxy[0] = 0;
		dwSize = sizeof(szProxy)/sizeof(szProxy[0]);
		RegQueryValueExW(hKey, L"AutoConfigURL", NULL, 0, (LPBYTE)&szProxy, &dwSize);
		if (wcslen(szProxy))
		{
			RegCloseKey(hKey);
			return szProxy;
		}
		dwData = 0;
		RegQueryValueExW(hKey, L"ProxyEnable", NULL, 0, (LPBYTE)&dwData, &dwSize);
		if (dwData == 0)
		{
			RegCloseKey(hKey);
			return L"";
		}
		szProxy[0] = 0;
		dwSize = sizeof(szProxy)/sizeof(szProxy[0]);
		RegQueryValueExW(hKey, L"ProxyServer", NULL, 0, (LPBYTE)&szProxy, &dwSize);
		if (wcslen(szProxy))
		{
			RegCloseKey(hKey);
			return szProxy;
		}
	}
	return szProxy;
}


BOOL SetWindowsProxy(WCHAR* szProxy, const WCHAR* szProxyInterface=NULL)
{
	INTERNET_PER_CONN_OPTION_LIST conn_options;
	BOOL    bReturn;
	DWORD   dwBufferSize = sizeof(conn_options);

	if (wcslen(szProxy) == 0)
	{
		conn_options.dwSize = dwBufferSize;
		conn_options.pszConnection = (WCHAR*)szProxyInterface;
		conn_options.dwOptionCount = 1;
		conn_options.pOptions = (INTERNET_PER_CONN_OPTION*)malloc(sizeof(INTERNET_PER_CONN_OPTION)*conn_options.dwOptionCount);
		conn_options.pOptions[0].dwOption = INTERNET_PER_CONN_FLAGS;
		conn_options.pOptions[0].Value.dwValue = PROXY_TYPE_DIRECT;
	}
	else if (wcsstr(szProxy, L"://") != NULL)
	{
		conn_options.dwSize = dwBufferSize;
		conn_options.pszConnection = (WCHAR*)szProxyInterface;
		conn_options.dwOptionCount = 3;
		conn_options.pOptions = (INTERNET_PER_CONN_OPTION*)malloc(sizeof(INTERNET_PER_CONN_OPTION)*conn_options.dwOptionCount);
		conn_options.pOptions[0].dwOption = INTERNET_PER_CONN_FLAGS;
		conn_options.pOptions[0].Value.dwValue = PROXY_TYPE_DIRECT | PROXY_TYPE_AUTO_PROXY_URL;
		conn_options.pOptions[1].dwOption = INTERNET_PER_CONN_AUTOCONFIG_URL;
		conn_options.pOptions[1].Value.pszValue = szProxy;
		conn_options.pOptions[2].dwOption = INTERNET_PER_CONN_PROXY_BYPASS;
		conn_options.pOptions[2].Value.pszValue = (LPWSTR)L"<local>";
	}
	else
	{
		conn_options.dwSize = dwBufferSize;
		conn_options.pszConnection = (WCHAR*)szProxyInterface;
		conn_options.dwOptionCount = 3;
		conn_options.pOptions = (INTERNET_PER_CONN_OPTION*)malloc(sizeof(INTERNET_PER_CONN_OPTION)*conn_options.dwOptionCount);
		conn_options.pOptions[0].dwOption = INTERNET_PER_CONN_FLAGS;
		conn_options.pOptions[0].Value.dwValue = PROXY_TYPE_DIRECT | PROXY_TYPE_PROXY;
		conn_options.pOptions[1].dwOption = INTERNET_PER_CONN_PROXY_SERVER;
		conn_options.pOptions[1].Value.pszValue = szProxy;
		conn_options.pOptions[2].dwOption = INTERNET_PER_CONN_PROXY_BYPASS;
		conn_options.pOptions[2].Value.pszValue = (LPWSTR)L"<local>";
	}

	bReturn = InternetSetOption(NULL, INTERNET_OPTION_PER_CONNECTION_OPTION, &conn_options, dwBufferSize);
	free(conn_options.pOptions);
	InternetSetOption(NULL, INTERNET_OPTION_SETTINGS_CHANGED, NULL, 0);
	InternetSetOption(NULL, INTERNET_OPTION_REFRESH , NULL, 0);
	return bReturn;
}

BOOL SetWindowsProxyForAllRasConnections(WCHAR* szProxy)
{
	for (LPCSTR lpRasPbk = szRasPbk; *lpRasPbk; lpRasPbk += strlen(lpRasPbk) + 1)
	{
		char szPath[2048] = "";
		if (ExpandEnvironmentStringsA(lpRasPbk, szPath, sizeof(szPath)/sizeof(szPath[0])))
		{
			char line[2048] = "";
			int length = 0;
			FILE * fp = fopen(szPath, "r");
			if (fp != NULL)
			{
				while (!feof(fp))
				{
					if (fgets(line, sizeof(line)/sizeof(line[0])-1, fp))
					{
						length = strlen(line);
						if (length > 3 && line[0] == '[' && line[length-2] == ']')
						{
							line[length-2] = 0;
							WCHAR szSection[64] = L"";
							MultiByteToWideChar(CP_UTF8, 0, line+1, -1, szSection, sizeof(szSection)/sizeof(szSection[0]));
							SetWindowsProxy(szProxy, szSection);
						}
					}
				}
				fclose(fp);
			}
		}
	}
	return TRUE;
}

BOOL ShowPopupMenu()
{
	POINT pt;
	HMENU hSubMenu = NULL;
	BOOL isZHCN = GetSystemDefaultLCID() == 2052;
	LPCTSTR lpCurrentProxy = GetWindowsProxy();
	if (lpProxyList[1] != NULL)
	{
		hSubMenu = CreatePopupMenu();
		for (int i = 0; lpProxyList[i]; i++)
		{
			UINT uFlags = wcscmp(lpProxyList[i], lpCurrentProxy) == 0 ? MF_STRING | MF_CHECKED : MF_STRING;
			LPCTSTR lpText = wcslen(lpProxyList[i]) ? lpProxyList[i] : ( isZHCN ? L"\x7981\x7528\x4ee3\x7406" : L"<None>");
			AppendMenu(hSubMenu, uFlags, WM_TASKBARNOTIFY_MENUITEM_PROXYLIST_BASE+i, lpText);
		}
	}

	HMENU hMenu = CreatePopupMenu();
	AppendMenu(hMenu, MF_STRING, WM_TASKBARNOTIFY_MENUITEM_SHOW, ( isZHCN ? L"\x663e\x793a" : L"Show") );
	AppendMenu(hMenu, MF_STRING, WM_TASKBARNOTIFY_MENUITEM_HIDE, ( isZHCN ? L"\x9690\x85cf" : L"Hide") );
	if (hSubMenu != NULL)
	{
		AppendMenu(hMenu, MF_STRING | MF_POPUP, (UINT_PTR)hSubMenu, ( isZHCN ? L"\x8bbe\x7f6e IE \x4ee3\x7406" : L"Set IE Proxy") );
	}
	AppendMenu(hMenu, MF_STRING, WM_TASKBARNOTIFY_MENUITEM_RELOAD, ( isZHCN ? L"\x91cd\x65b0\x8f7d\x5165" : L"Reload") );
	AppendMenu(hMenu, MF_STRING, WM_TASKBARNOTIFY_MENUITEM_EXIT,   ( isZHCN ? L"\x9000\x51fa" : L"Exit") );
	GetCursorPos(&pt);
	TrackPopupMenu(hMenu, TPM_LEFTALIGN, pt.x, pt.y, 0, hWnd, NULL);
	PostMessage(hWnd, WM_NULL, 0, 0);
	if (hSubMenu != NULL)
		DestroyMenu(hSubMenu);
	DestroyMenu(hMenu);
	return TRUE;
}

BOOL ParseProxyList()
{
	WCHAR * tmpProxyString = _wcsdup(szProxyString);
	ExpandEnvironmentStrings(tmpProxyString, szProxyString, sizeof(szProxyString)/sizeof(szProxyString[0]));
	free(tmpProxyString);
	const WCHAR *sep = L"\n";
	WCHAR *pos = wcstok(szProxyString, sep);
	UINT i = 0;
	lpProxyList[i++] = (LPWSTR)L"";
	while (pos && i < sizeof(lpProxyList)/sizeof(lpProxyList[0]))
	{
		lpProxyList[i++] = pos;
		pos = wcstok(NULL, sep);
	}
	lpProxyList[i] = 0;


	for (LPSTR ptr = szRasPbk; *ptr; ptr++)
	{
		if (*ptr == '\n')
		{
			*ptr++ = 0;
		}
	}
	return TRUE;
}

BOOL InitInstance(HINSTANCE hInstance, int nCmdShow)
{
   hWnd = CreateWindow(szWindowClass, szTitle, WS_OVERLAPPED|WS_SYSMENU,
	  NULL, NULL, CW_USEDEFAULT, CW_USEDEFAULT, NULL, NULL, hInstance, NULL);

   if (!hWnd)
   {
	  return FALSE;
   }

   ShowWindow(hWnd, nCmdShow);
   UpdateWindow(hWnd);

   return TRUE;
}

BOOL CDCurrentDirectory()
{
	WCHAR szPath[4096] = L"";
	GetModuleFileName(NULL, szPath, sizeof(szPath)/sizeof(szPath[0])-1);
	*wcsrchr(szPath, L'\\') = 0;
	SetCurrentDirectory(szPath);
	SetEnvironmentVariableW(L"CWD", szPath);
	return TRUE;
}

BOOL SetEenvironment()
{
	LoadString(hInst, IDS_CMDLINE, szCommandLine, sizeof(szCommandLine)/sizeof(szCommandLine[0])-1);
	LoadString(hInst, IDS_ENVIRONMENT, szEnvironment, sizeof(szEnvironment)/sizeof(szEnvironment[0])-1);
	LoadString(hInst, IDS_PROXYLIST, szProxyString, sizeof(szProxyString)/sizeof(szEnvironment[0])-1);
	LoadStringA(hInst, IDS_RASPBK, szRasPbk, sizeof(szRasPbk)/sizeof(szRasPbk[0])-1);

	const WCHAR *sep = L"\n";
	WCHAR *pos = NULL;
	WCHAR *token = wcstok(szEnvironment, sep);
	while(token != NULL)
	{
		if ((pos = wcschr(token, L'=')) != NULL)
		{
			*pos = 0;
			SetEnvironmentVariableW(token, pos+1);
			//wprintf(L"[%s] = [%s]\n", token, pos+1);
		}
		token = wcstok(NULL, sep);
	}

	GetEnvironmentVariableW(L"TASKBAR_TITLE", szTitle, sizeof(szTitle)/sizeof(szTitle[0])-1);
	GetEnvironmentVariableW(L"TASKBAR_TOOLTIP", szTooltip, sizeof(szTooltip)/sizeof(szTooltip[0])-1);
	GetEnvironmentVariableW(L"TASKBAR_BALLOON", szBalloon, sizeof(szBalloon)/sizeof(szBalloon[0])-1);

	return TRUE;
}

BOOL WINAPI ConsoleHandler(DWORD CEvent)
{
	switch(CEvent)
	{
	case CTRL_LOGOFF_EVENT:
	case CTRL_SHUTDOWN_EVENT:
	case CTRL_CLOSE_EVENT:
		SendMessage(hWnd, WM_CLOSE, NULL, NULL);
		break;
	}
	return TRUE;
}

BOOL CreateConsole()
{
	WCHAR szVisible[BUFSIZ] = L"";

	AllocConsole();
	_wfreopen(L"CONIN$",  L"r+t", stdin);
	_wfreopen(L"CONOUT$", L"w+t", stdout);

	hConsole = GetConsoleWindow();

	if (GetEnvironmentVariableW(L"TASKBAR_VISIBLE", szVisible, BUFSIZ-1) && szVisible[0] == L'0')
	{
		ShowWindow(hConsole, SW_HIDE);
	}
	else
	{
		SetForegroundWindow(hConsole);
	}

	if (SetConsoleCtrlHandler((PHANDLER_ROUTINE)ConsoleHandler,TRUE)==FALSE)
	{
		printf("Unable to install handler!\n");
		return FALSE;
	}

	CONSOLE_SCREEN_BUFFER_INFO csbi;
	if (GetConsoleScreenBufferInfo(GetStdHandle(STD_ERROR_HANDLE), &csbi))
	{
		COORD size = csbi.dwSize;
		if (size.Y < 2048)
		{
			size.Y = 2048;
			if (!SetConsoleScreenBufferSize(GetStdHandle(STD_ERROR_HANDLE), size))
			{
				printf("Unable to set console screen buffer size!\n");
			}
		}
	}

	return TRUE;
}

BOOL ExecCmdline()
{
	SetWindowText(hConsole, szTitle);
	STARTUPINFO si = { sizeof(si) };
	PROCESS_INFORMATION pi;
	si.dwFlags = STARTF_USESHOWWINDOW;
	si.wShowWindow = TRUE;
	BOOL bRet = CreateProcess(NULL, szCommandLine, NULL, NULL, FALSE, NULL, NULL, NULL, &si, &pi);
	if(bRet)
	{
		dwChildrenPid = MyGetProcessId(pi.hProcess);
	}
	else
	{
		wprintf(L"ExecCmdline \"%s\" failed!\n", szCommandLine);
		MessageBox(NULL, szCommandLine, L"Error: Cannot execute!", MB_OK);
		ExitProcess(0);
	}
	CloseHandle(pi.hThread);
	CloseHandle(pi.hProcess);
	return TRUE;
}

BOOL TryDeleteUpdateFiles()
{
	WIN32_FIND_DATA FindFileData;
	HANDLE hFind;

	hFind = FindFirstFile(L"~*.tmp", &FindFileData);
	if (hFind == INVALID_HANDLE_VALUE)
	{
		return TRUE;
	}

	do
	{
		DeleteFile(FindFileData.cFileName);
		if (!FindNextFile(hFind, &FindFileData))
		{
			break;
		}
	} while(TRUE);
	FindClose(hFind);

	return TRUE;
}

BOOL ReloadCmdline()
{
	//HANDLE hProcess = OpenProcess(SYNCHRONIZE|PROCESS_TERMINATE, FALSE, dwChildrenPid);
	//if (hProcess)
	//{
	//	TerminateProcess(hProcess, 0);
	//}
	ShowWindow(hConsole, SW_SHOW);
	SetForegroundWindow(hConsole);
	wprintf(L"\n\n");
	MyEndTask(dwChildrenPid);
	wprintf(L"\n\n");
	Sleep(200);
	ExecCmdline();
	return TRUE;
}

LRESULT CALLBACK WndProc(HWND hWnd, UINT message, WPARAM wParam, LPARAM lParam)
{
	static const UINT WM_TASKBARCREATED = ::RegisterWindowMessage(L"TaskbarCreated");
	UINT nID;
	switch (message)
	{
		case WM_TASKBARNOTIFY:
			if (lParam == WM_LBUTTONUP)
			{
				ShowWindow(hConsole, !IsWindowVisible(hConsole));
				SetForegroundWindow(hConsole);
			}
			else if (lParam == WM_RBUTTONUP)
			{
				SetForegroundWindow(hWnd);
				ShowPopupMenu();
			}
			break;
		case WM_COMMAND:
			nID = LOWORD(wParam);
			if (nID == WM_TASKBARNOTIFY_MENUITEM_SHOW)
			{
				ShowWindow(hConsole, SW_SHOW);
				SetForegroundWindow(hConsole);
			}
			else if (nID == WM_TASKBARNOTIFY_MENUITEM_HIDE)
			{
				ShowWindow(hConsole, SW_HIDE);
			}
			if (nID == WM_TASKBARNOTIFY_MENUITEM_RELOAD)
			{
				ReloadCmdline();
			}
			else if (nID == WM_TASKBARNOTIFY_MENUITEM_ABOUT)
			{
				MessageBoxW(hWnd, szTooltip, szWindowClass, 0);
			}
			else if (nID == WM_TASKBARNOTIFY_MENUITEM_EXIT)
			{
				DeleteTrayIcon();
				PostMessage(hConsole, WM_CLOSE, 0, 0);
			}
			else if (WM_TASKBARNOTIFY_MENUITEM_PROXYLIST_BASE <= nID && nID <= WM_TASKBARNOTIFY_MENUITEM_PROXYLIST_BASE+sizeof(lpProxyList)/sizeof(lpProxyList[0]))
			{
				WCHAR *szProxy = lpProxyList[nID-WM_TASKBARNOTIFY_MENUITEM_PROXYLIST_BASE];
				SetWindowsProxy(szProxy);
				SetWindowsProxyForAllRasConnections(szProxy);
				ShowTrayIcon(szProxy, NIM_MODIFY);
			}
			break;
		case WM_CLOSE:
			DeleteTrayIcon();
			PostQuitMessage(0);
			break;
		case WM_DESTROY:
			PostQuitMessage(0);
			break;
		default:
			if (message == WM_TASKBARCREATED)
			{
				ShowTrayIcon(NULL, NIM_ADD);
				break;
			}
			return DefWindowProc(hWnd, message, wParam, lParam);
   }
   return 0;
}

ATOM MyRegisterClass(HINSTANCE hInstance)
{
	WNDCLASSEX wcex;

	wcex.cbSize = sizeof(WNDCLASSEX);

	wcex.style			= CS_HREDRAW | CS_VREDRAW;
	wcex.lpfnWndProc	= (WNDPROC)WndProc;
	wcex.cbClsExtra		= 0;
	wcex.cbWndExtra		= 0;
	wcex.hInstance		= hInstance;
	wcex.hIcon			= LoadIcon(hInstance, (LPCTSTR)IDI_TASKBAR);
	wcex.hCursor		= LoadCursor(NULL, IDC_ARROW);
	wcex.hbrBackground	= (HBRUSH)(COLOR_WINDOW+1);
	wcex.lpszMenuName	= (LPCTSTR)NULL;
	wcex.lpszClassName	= szWindowClass;
	wcex.hIconSm		= LoadIcon(wcex.hInstance, (LPCTSTR)IDI_SMALL);

	return RegisterClassEx(&wcex);
}

int APIENTRY WinMain(HINSTANCE hInstance, HINSTANCE, LPSTR lpCmdLine, int nCmdShow)
{
	MSG msg;
	hInst = hInstance;
	CDCurrentDirectory();
	SetEenvironment();
	ParseProxyList();
	MyRegisterClass(hInstance);
	if (!InitInstance (hInstance, SW_HIDE))
	{
		return FALSE;
	}
	CreateConsole();
	ExecCmdline();
	ShowTrayIcon(GetWindowsProxy());
	TryDeleteUpdateFiles();
	while (GetMessage(&msg, NULL, 0, 0))
	{
		TranslateMessage(&msg);
		DispatchMessage(&msg);
	}
	return 0;
}

