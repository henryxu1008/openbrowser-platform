using System;
using System.Runtime.InteropServices;
using System.Runtime.InteropServices.ComTypes;
using System.Text;

namespace Shuomi {
    // Use the Windows Unicode Shell Link API rather than the ANSI WScript shortcut API.
    // https://learn.microsoft.com/windows/win32/shell/links
    [ComImport, Guid("00021401-0000-0000-C000-000000000046")]
    internal class ShellLink { }

    [ComImport, Guid("000214F9-0000-0000-C000-000000000046"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
    internal interface IShellLinkW {
        void GetPath([Out, MarshalAs(UnmanagedType.LPWStr)] StringBuilder path, int length, IntPtr findData, uint flags);
        void GetIDList(out IntPtr list);
        void SetIDList(IntPtr list);
        void GetDescription([Out, MarshalAs(UnmanagedType.LPWStr)] StringBuilder description, int length);
        void SetDescription([MarshalAs(UnmanagedType.LPWStr)] string description);
        void GetWorkingDirectory([Out, MarshalAs(UnmanagedType.LPWStr)] StringBuilder directory, int length);
        void SetWorkingDirectory([MarshalAs(UnmanagedType.LPWStr)] string directory);
        void GetArguments([Out, MarshalAs(UnmanagedType.LPWStr)] StringBuilder arguments, int length);
        void SetArguments([MarshalAs(UnmanagedType.LPWStr)] string arguments);
        void GetHotkey(out short hotkey);
        void SetHotkey(short hotkey);
        void GetShowCmd(out int command);
        void SetShowCmd(int command);
        void GetIconLocation([Out, MarshalAs(UnmanagedType.LPWStr)] StringBuilder icon, int length, out int index);
        void SetIconLocation([MarshalAs(UnmanagedType.LPWStr)] string icon, int index);
        void SetRelativePath([MarshalAs(UnmanagedType.LPWStr)] string path, uint reserved);
        void Resolve(IntPtr window, uint flags);
        void SetPath([MarshalAs(UnmanagedType.LPWStr)] string path);
    }

    public sealed class ShortcutInfo {
        public string TargetPath { get; set; }
        public string Arguments { get; set; }
    }

    public static class ShortcutFile {
        public static ShortcutInfo Read(string path) {
            var link = (IShellLinkW)new ShellLink();
            try {
                ((IPersistFile)link).Load(path, 0);
                var target = new StringBuilder(32768);
                var arguments = new StringBuilder(32768);
                link.GetPath(target, target.Capacity, IntPtr.Zero, 4); // SLGP_RAWPATH, no resolution or launch.
                link.GetArguments(arguments, arguments.Capacity);
                return new ShortcutInfo { TargetPath = target.ToString(), Arguments = arguments.ToString() };
            } finally { Marshal.FinalReleaseComObject(link); }
        }

        public static void Write(string path, string target, string arguments, string directory, string icon) {
            var link = (IShellLinkW)new ShellLink();
            try {
                link.SetPath(target);
                link.SetArguments(arguments);
                link.SetWorkingDirectory(directory);
                link.SetDescription("Browser shortcut");
                link.SetIconLocation(icon, 0);
                ((IPersistFile)link).Save(path, true);
            } finally { Marshal.FinalReleaseComObject(link); }
        }
    }
}
